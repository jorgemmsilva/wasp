// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package chains

import (
	"context"
	"sync"
	"time"

	"golang.org/x/xerrors"

	"github.com/iotaledger/hive.go/core/logger"
	"github.com/iotaledger/wasp/packages/chain"
	"github.com/iotaledger/wasp/packages/database/dbmanager"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/metrics"
	"github.com/iotaledger/wasp/packages/metrics/nodeconnmetrics"
	"github.com/iotaledger/wasp/packages/peering"
	"github.com/iotaledger/wasp/packages/registry"
	"github.com/iotaledger/wasp/packages/state"
	"github.com/iotaledger/wasp/packages/vm/processors"
)

type Provider func() *Chains

func (chains Provider) ChainProvider() func(chainID *isc.ChainID) chain.Chain {
	return func(chainID *isc.ChainID) chain.Chain {
		return chains().Get(chainID)
	}
}

type ChainProvider func(chainID *isc.ChainID) chain.Chain

type Chains struct {
	ctx                              context.Context
	log                              *logger.Logger
	nodeConnection                   chain.NodeConnection
	processorConfig                  *processors.Config
	offledgerBroadcastUpToNPeers     int
	offledgerBroadcastInterval       time.Duration
	pullMissingRequestsFromCommittee bool
	networkProvider                  peering.NetworkProvider
	getOrCreateKVStore               dbmanager.ChainKVStoreProvider
	rawBlocksEnabled                 bool
	rawBlocksDir                     string
	registry                         registry.Registry
	metrics                          *metrics.Metrics

	mutex     sync.RWMutex
	allChains map[isc.ChainID]*activeChain
}

type activeChain struct {
	chain      chain.Chain
	cancelFunc context.CancelFunc
}

func New(
	log *logger.Logger,
	nodeConnection chain.NodeConnection,
	processorConfig *processors.Config,
	offledgerBroadcastUpToNPeers int,
	offledgerBroadcastInterval time.Duration,
	pullMissingRequestsFromCommittee bool,
	networkProvider peering.NetworkProvider,
	getOrCreateKVStore dbmanager.ChainKVStoreProvider,
	rawBlocksEnabled bool,
	rawBlocksDir string,
) *Chains {
	ret := &Chains{
		log:                              log,
		allChains:                        map[isc.ChainID]*activeChain{},
		nodeConnection:                   nodeConnection,
		processorConfig:                  processorConfig,
		offledgerBroadcastUpToNPeers:     offledgerBroadcastUpToNPeers,
		offledgerBroadcastInterval:       offledgerBroadcastInterval,
		pullMissingRequestsFromCommittee: pullMissingRequestsFromCommittee,
		networkProvider:                  networkProvider,
		getOrCreateKVStore:               getOrCreateKVStore,
		rawBlocksEnabled:                 rawBlocksEnabled,
		rawBlocksDir:                     rawBlocksDir,
	}
	return ret
}

func (c *Chains) Run(
	ctx context.Context,
	allRegistries registry.Registry, // TODO: Move this to the constructor?
	allMetrics *metrics.Metrics, // TODO: Move this to the constructor?
) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if c.ctx == nil {
		c.log.Warnf("Chains already running.")
		return nil
	}
	c.ctx = ctx
	c.registry = allRegistries
	c.metrics = allMetrics

	return c.activateAllFromRegistry() //nolint:contextcheck
}

// TODO: Why do we take these parameters here, and not in the constructor?
func (c *Chains) activateAllFromRegistry() error {
	chainRecords, err := c.registry.GetChainRecords()
	if err != nil {
		return xerrors.Errorf("cannot read chain records: %w", err)
	}

	astr := make([]string, len(chainRecords))
	for i := range astr {
		astr[i] = chainRecords[i].ChainID.String()[:10] + ".."
	}
	c.log.Debugf("loaded %d chain record(s) from registry: %+v", len(chainRecords), astr)

	for _, chainRecord := range chainRecords {
		if _, ok := c.allChains[chainRecord.ChainID]; !ok && chainRecord.Active {
			if err := c.Activate(&chainRecord.ChainID); err != nil {
				c.log.Errorf("cannot activate chain %s: %v", chainRecord.ChainID, err)
			}
		}
	}
	return nil
}

// Activate activates chain on the Wasp node.
func (c *Chains) Activate(chainID *isc.ChainID) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if c.ctx == nil {
		return xerrors.Errorf("run chains first")
	}
	//
	// Check, maybe it is already running.
	if _, ok := c.allChains[*chainID]; ok {
		c.log.Debugf("Chain %v is already activated", chainID.String())
		return nil
	}
	//
	// Activate the chain in the persistent store, if it is not activated yet.
	chainRecord, err := c.registry.GetChainRecordByChainID(chainID)
	if err != nil {
		return xerrors.Errorf("cannot get chain record for %v: %w", chainID, err)
	}
	if !chainRecord.Active {
		if _, err := c.registry.ActivateChainRecord(chainID); err != nil {
			return xerrors.Errorf("cannot activate chain: %w", err)
		}
	}
	//
	// Load or initialize new chain store.
	chainKVStore := c.getOrCreateKVStore(chainID)
	chainStore := state.NewStore(chainKVStore)
	chainState, err := chainStore.LatestState()
	chainIDInState, errChainID := chainState.Has(state.KeyChainID)
	if err != nil || errChainID != nil || !chainIDInState {
		chainStore = state.InitChainStore(chainKVStore)
	}
	// TODO: chainMetrics := c.allMetrics.NewChainMetrics(&chr.ChainID)
	chainCtx, chainCancel := context.WithCancel(c.ctx)
	newChain, err := chain.New(
		chainCtx,
		chainID,
		chainStore,
		nil, // TODO: c.nodeConnection,
		c.registry.GetNodeIdentity(),
		c.processorConfig,
		c.networkProvider,
		c.log,
	)
	if err != nil {
		chainCancel()
		return xerrors.Errorf("Chains.Activate: failed to create chain object: %w", err)
	}
	c.allChains[*chainID] = &activeChain{
		chain:      newChain,
		cancelFunc: chainCancel,
	}
	c.log.Infof("activated chain: %s", chainID.String())
	return nil
}

// Deactivate chain in the node.
func (c *Chains) Deactivate(chainID *isc.ChainID) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if _, err := c.registry.DeactivateChainRecord(chainID); err != nil {
		return xerrors.Errorf("cannot deactivate chain %v: %w", chainID, err)
	}

	ch, ok := c.allChains[*chainID]
	if !ok {
		c.log.Debugf("chain is not active: %s", chainID.String())
		return nil
	}
	ch.cancelFunc()
	delete(c.allChains, *chainID)
	c.log.Debugf("chain has been deactivated: %s", chainID.String())
	return nil
}

// Get returns active chain object or nil if it doesn't exist
// lazy unsubscribing
func (c *Chains) Get(chainID *isc.ChainID) chain.Chain {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	ret, ok := c.allChains[*chainID]
	if !ok {
		return nil
	}
	return ret
}

func (c *Chains) GetNodeConnectionMetrics() nodeconnmetrics.NodeConnectionMetrics {
	return c.nodeConnection.GetMetrics()
}
