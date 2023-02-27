// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package distSync

import (
	"fmt"
	"math/rand"

	"golang.org/x/exp/slices"

	"github.com/iotaledger/hive.go/core/logger"
	"github.com/iotaledger/wasp/packages/gpa"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/util"
)

const (
	maxTTL byte = 1
)

// The implementation is trivial and naive for now. A proper gossip or structured
// broadcast (possibly a mix of both) should be implemented here.
//
// In the current algorithm, for sharing a message:
//   - Just send a message to all the committee nodes.
//
// For querying a message:
//   - First ask all the committee for the message.
//   - If response not received, ask random subsets of server nodes.
//
// TODO: For the future releases: Implement proper dissemination algorithm.
type distSyncImpl struct {
	me                gpa.NodeID
	serverNodes       []gpa.NodeID // Should be used to push and query for requests.
	accessNodes       []gpa.NodeID // Maybe is not needed? Lets keep it until the redesign.
	committeeNodes    []gpa.NodeID // Subset of serverNodes and accessNodes.
	requestNeededCB   func(*isc.RequestRef) isc.Request
	requestReceivedCB func(isc.Request)
	nodeCountToShare  int // Number of nodes to share a request per iteration.
	maxMsgsPerTick    int
	needed            map[isc.RequestRefKey]*isc.RequestRef
	rnd               *rand.Rand
	log               *logger.Logger
}

var _ gpa.GPA = &distSyncImpl{}

func New(
	me gpa.NodeID,
	requestNeededCB func(*isc.RequestRef) isc.Request,
	requestReceivedCB func(isc.Request),
	maxMsgsPerTick int,
	log *logger.Logger,
) gpa.GPA {
	return &distSyncImpl{
		me:                me,
		serverNodes:       []gpa.NodeID{},
		accessNodes:       []gpa.NodeID{},
		committeeNodes:    []gpa.NodeID{},
		requestNeededCB:   requestNeededCB,
		requestReceivedCB: requestReceivedCB,
		nodeCountToShare:  0,
		maxMsgsPerTick:    maxMsgsPerTick,
		needed:            map[isc.RequestRefKey]*isc.RequestRef{},
		rnd:               util.NewPseudoRand(),
		log:               log,
	}
}

func (dsi *distSyncImpl) Input(input gpa.Input) gpa.OutMessages {
	dsi.log.Debugf("Input %T: %+v", input, input)
	switch input := input.(type) {
	case *inputServerNodes:
		return dsi.handleInputServerNodes(input)
	case *inputAccessNodes:
		return dsi.handleInputAccessNodes(input)
	case *inputPublishRequest:
		return dsi.handleInputPublishRequest(input)
	case *inputRequestNeeded:
		return dsi.handleInputRequestNeeded(input)
	case *inputTimeTick:
		return dsi.handleInputTimeTick()
	}
	panic(fmt.Errorf("unexpected input type %T: %+v", input, input))
}

func (dsi *distSyncImpl) Message(msg gpa.Message) gpa.OutMessages {
	switch msg := msg.(type) {
	case *msgMissingRequest:
		return dsi.handleMsgMissingRequest(msg)
	case *msgShareRequest:
		return dsi.handleMsgShareRequest(msg)
	}
	dsi.log.Warnf("unexpected message %T: %+v", msg, msg)
	return nil
}

func (dsi *distSyncImpl) Output() gpa.Output {
	return nil // Output is provided via callbacks.
}

func (dsi *distSyncImpl) StatusString() string {
	return fmt.Sprintf("{MP, neededReqs=%v, nodeCountToShare=%v}", len(dsi.needed), dsi.nodeCountToShare)
}

func (dsi *distSyncImpl) handleInputServerNodes(input *inputServerNodes) gpa.OutMessages {
	dsi.log.Debugf("handleInputServerNodes: %v", input)
	dsi.handleCommitteeNodes(input.committeeNodes)
	dsi.serverNodes = input.serverNodes
	for i := range dsi.committeeNodes { // Ensure server nodes contain the committee nodes.
		if slices.Index(dsi.serverNodes, dsi.committeeNodes[i]) == -1 {
			dsi.serverNodes = append(dsi.serverNodes, dsi.committeeNodes[i])
		}
	}
	return dsi.handleInputTimeTick() // Re-send requests if node set has changed.
}

func (dsi *distSyncImpl) handleInputAccessNodes(input *inputAccessNodes) gpa.OutMessages {
	dsi.log.Debugf("handleInputAccessNodes: %v", input)
	dsi.handleCommitteeNodes(input.committeeNodes)
	dsi.accessNodes = input.accessNodes
	for i := range dsi.committeeNodes { // Ensure access nodes contain the committee nodes.
		if slices.Index(dsi.accessNodes, dsi.committeeNodes[i]) == -1 {
			dsi.accessNodes = append(dsi.accessNodes, dsi.committeeNodes[i])
		}
	}
	return dsi.handleInputTimeTick() // Re-send requests if node set has changed.
}

func (dsi *distSyncImpl) handleCommitteeNodes(committeeNodes []gpa.NodeID) {
	dsi.committeeNodes = committeeNodes
	dsi.nodeCountToShare = (len(dsi.committeeNodes)-1)/3 + 1 // F+1
	if dsi.nodeCountToShare < 2 {
		dsi.nodeCountToShare = 2
	}
	if dsi.nodeCountToShare > len(dsi.committeeNodes) {
		dsi.nodeCountToShare = len(dsi.committeeNodes)
	}
}

// In the current algorithm, for sharing a message:
//   - Just send a message to all the committee nodes (or server nodes, if committee is not known).
func (dsi *distSyncImpl) handleInputPublishRequest(input *inputPublishRequest) gpa.OutMessages {
	msgs := gpa.NoMessages()
	var publishToNodes []gpa.NodeID
	if len(dsi.committeeNodes) > 0 {
		publishToNodes = dsi.committeeNodes
	} else {
		publishToNodes = dsi.serverNodes
	}
	for _, node := range publishToNodes {
		msgs.Add(newMsgShareRequest(input.request, 0, node))
	}
	//
	// Delete the it from the "needed" list, if any.
	// This node has the request, if it tries to publish it.
	reqRef := isc.RequestRefFromRequest(input.request)
	delete(dsi.needed, reqRef.AsKey())
	return msgs
}

// For querying a message:
//   - First ask all the committee for the message.
//   - ...
func (dsi *distSyncImpl) handleInputRequestNeeded(input *inputRequestNeeded) gpa.OutMessages {
	reqRefKey := input.requestRef.AsKey()
	if !input.needed {
		delete(dsi.needed, reqRefKey)
		return nil
	}
	dsi.needed[reqRefKey] = input.requestRef
	msgs := gpa.NoMessages()
	for _, nid := range dsi.committeeNodes {
		msgs.Add(newMsgMissingRequest(input.requestRef, nid))
	}
	return msgs
}

// For querying a message:
//   - ...
//   - If response not received, ask random subsets of server nodes.
func (dsi *distSyncImpl) handleInputTimeTick() gpa.OutMessages {
	if len(dsi.needed) == 0 {
		return nil
	}
	nodeCount := len(dsi.serverNodes)
	if nodeCount == 0 {
		return nil
	}
	msgs := gpa.NoMessages()
	nodePerm := dsi.rnd.Perm(nodeCount)
	counter := 0
	for _, reqRef := range dsi.needed { // Access is randomized.
		msgs.Add(newMsgMissingRequest(reqRef, dsi.serverNodes[nodePerm[counter%nodeCount]]))
		counter++
		if counter > dsi.maxMsgsPerTick {
			break
		}
	}
	return msgs
}

func (dsi *distSyncImpl) handleMsgMissingRequest(msg *msgMissingRequest) gpa.OutMessages {
	req := dsi.requestNeededCB(msg.requestRef)
	if req != nil {
		msgs := gpa.NoMessages()
		msgs.Add(newMsgShareRequest(req, 0, msg.Sender()))
		return msgs
	}
	return nil
}

func (dsi *distSyncImpl) handleMsgShareRequest(msg *msgShareRequest) gpa.OutMessages {
	reqRefKey := isc.RequestRefFromRequest(msg.request).AsKey()
	dsi.requestReceivedCB(msg.request)
	delete(dsi.needed, reqRefKey)
	if msg.ttl > 0 {
		ttl := msg.ttl
		if ttl > maxTTL {
			ttl = maxTTL
		}
		msgs := gpa.NoMessages()
		perm := dsi.rnd.Perm(len(dsi.committeeNodes))
		for i := 0; i < dsi.nodeCountToShare; i++ {
			msgs.Add(newMsgShareRequest(msg.request, ttl-1, dsi.committeeNodes[perm[i]]))
		}
		return msgs
	}
	return nil
}
