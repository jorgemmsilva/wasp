// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package mempool

import (
	"fmt"
	"io"
	"time"

	"github.com/iotaledger/hive.go/ds/shrinkingmap"
	"github.com/iotaledger/hive.go/logger"
	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv/codec"
)

type typedPool[V isc.Request] struct {
	waitReq       WaitReq
	requests      *shrinkingmap.ShrinkingMap[isc.RequestRefKey, *typedPoolEntry[V]]
	sizeMetric    func(int)
	timeMetric    func(time.Duration)
	log           *logger.Logger
	l1APIProvider iotago.APIProvider
}

type typedPoolEntry[V isc.Request] struct {
	req V
	ts  time.Time
}

var _ RequestPool[isc.OffLedgerRequest] = &typedPool[isc.OffLedgerRequest]{}

func NewTypedPool[V isc.Request](waitReq WaitReq, l1APIProvider iotago.APIProvider, sizeMetric func(int), timeMetric func(time.Duration), log *logger.Logger) RequestPool[V] {
	return &typedPool[V]{
		waitReq:       waitReq,
		l1APIProvider: l1APIProvider,
		requests:      shrinkingmap.New[isc.RequestRefKey, *typedPoolEntry[V]](),
		sizeMetric:    sizeMetric,
		timeMetric:    timeMetric,
		log:           log,
	}
}

func (olp *typedPool[V]) Has(reqRef *isc.RequestRef) bool {
	return olp.requests.Has(reqRef.AsKey())
}

func (olp *typedPool[V]) Get(reqRef *isc.RequestRef) V {
	entry, exists := olp.requests.Get(reqRef.AsKey())
	if !exists {
		return *new(V)
	}
	return entry.req
}

func (olp *typedPool[V]) Add(request V) {
	refKey := isc.RequestRefFromRequest(request).AsKey()
	if olp.requests.Set(refKey, &typedPoolEntry[V]{req: request, ts: time.Now()}) {
		olp.log.Debugf("ADD %v as key=%v", request.ID(), refKey)
		olp.sizeMetric(olp.requests.Size())
	}
	olp.waitReq.MarkAvailable(request)
}

func (olp *typedPool[V]) Remove(request V) {
	refKey := isc.RequestRefFromRequest(request).AsKey()
	if entry, ok := olp.requests.Get(refKey); ok {
		if olp.requests.Delete(refKey) {
			olp.log.Debugf("DEL %v as key=%v", request.ID(), refKey)
		}
		olp.sizeMetric(olp.requests.Size())
		olp.timeMetric(time.Since(entry.ts))
	}
}

func (olp *typedPool[V]) Filter(predicate func(request V, ts time.Time) bool) {
	olp.requests.ForEach(func(refKey isc.RequestRefKey, entry *typedPoolEntry[V]) bool {
		if !predicate(entry.req, entry.ts) {
			if olp.requests.Delete(refKey) {
				olp.log.Debugf("DEL %v as key=%v", entry.req.ID(), refKey)
				olp.timeMetric(time.Since(entry.ts))
			}
		}
		return true
	})
	olp.sizeMetric(olp.requests.Size())
}

func (olp *typedPool[V]) Iterate(f func(e *typedPoolEntry[V])) {
	olp.requests.ForEach(func(refKey isc.RequestRefKey, entry *typedPoolEntry[V]) bool {
		f(entry)
		return true
	})
	olp.sizeMetric(olp.requests.Size())
}

func (olp *typedPool[V]) StatusString() string {
	return fmt.Sprintf("{|req|=%d}", olp.requests.Size())
}

func (olp *typedPool[V]) WriteContent(w io.Writer) {
	olp.requests.ForEach(func(_ isc.RequestRefKey, entry *typedPoolEntry[V]) bool {
		jsonData, err := isc.RequestToJSON(entry.req, olp.l1APIProvider.LatestAPI())
		if err != nil {
			return false // stop iteration
		}
		_, err = w.Write(codec.Uint32.Encode(uint32(len(jsonData))))
		if err != nil {
			return false // stop iteration
		}
		_, err = w.Write(jsonData)
		return err == nil
	})
}
