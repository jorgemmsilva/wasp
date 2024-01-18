package websocket

import (
	"context"

	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/event"
	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/publisher"
	"github.com/iotaledger/wasp/packages/webapi/models"
)

type ISCEvent struct {
	Kind      publisher.ISCEventType `json:"kind"`
	Issuer    string                 `json:"issuer"`    // (isc.AgentID) nil means issued by the VM
	RequestID string                 `json:"requestID"` // (isc.RequestID)
	ChainID   string                 `json:"chainID"`   // (isc.ChainID)
	Payload   any                    `json:"payload"`
}

func MapISCEvent[T any](iscEvent *publisher.ISCEvent[T], mappedPayload any, l1API iotago.API) *ISCEvent {
	issuer := iscEvent.Issuer.Bech32(l1API.ProtocolParameters().Bech32HRP())

	if issuer == "-" {
		// If the agentID is nil, it should be empty in the JSON result, not '-'
		issuer = ""
	}

	return &ISCEvent{
		Kind:      iscEvent.Kind,
		ChainID:   iscEvent.ChainID.Bech32(l1API.ProtocolParameters().Bech32HRP()),
		RequestID: iscEvent.RequestID.String(),
		Issuer:    issuer,
		Payload:   mappedPayload,
	}
}

type EventHandler struct {
	publisher             *publisher.Publisher
	publishEvent          *event.Event1[*ISCEvent]
	subscriptionValidator *SubscriptionValidator
	l1Api                 iotago.API
}

func NewEventHandler(pub *publisher.Publisher, publishEvent *event.Event1[*ISCEvent], subscriptionValidator *SubscriptionValidator, l1Api iotago.API) *EventHandler {
	return &EventHandler{
		publisher:             pub,
		publishEvent:          publishEvent,
		subscriptionValidator: subscriptionValidator,
		l1Api:                 l1Api,
	}
}

func (p *EventHandler) AttachToEvents() context.CancelFunc {
	return lo.Batch(

		p.publisher.Events.NewBlock.Hook(func(block *publisher.ISCEvent[*publisher.BlockWithTrieRoot]) {
			if !p.subscriptionValidator.shouldProcessEvent(block.ChainID.Bech32(p.l1Api.ProtocolParameters().Bech32HRP()), block.Kind) {
				return
			}

			blockInfo := models.MapBlockInfoResponse(block.Payload.BlockInfo)
			iscEvent := MapISCEvent(block, blockInfo, p.l1Api)
			p.publishEvent.Trigger(iscEvent)
		}).Unhook,

		p.publisher.Events.RequestReceipt.Hook(func(block *publisher.ISCEvent[*publisher.ReceiptWithError]) {
			if !p.subscriptionValidator.shouldProcessEvent(block.ChainID.Bech32(p.l1Api.ProtocolParameters().Bech32HRP()), block.Kind) {
				return
			}

			receipt := models.MapReceiptResponse(p.l1Api, block.Payload.RequestReceipt)
			iscEvent := MapISCEvent(block, receipt, p.l1Api)
			p.publishEvent.Trigger(iscEvent)
		}).Unhook,

		p.publisher.Events.BlockEvents.Hook(func(block *publisher.ISCEvent[[]*isc.Event]) {
			if !p.subscriptionValidator.shouldProcessEvent(block.ChainID.Bech32(p.l1Api.ProtocolParameters().Bech32HRP()), block.Kind) {
				return
			}

			iscEvent := MapISCEvent(block, block.Payload, p.l1Api)
			p.publishEvent.Trigger(iscEvent)
		}).Unhook,
	)
}
