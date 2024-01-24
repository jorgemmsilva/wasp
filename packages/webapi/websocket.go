package webapi

import (
	_ "embed"
	"fmt"

	"github.com/pangpanglabs/echoswagger/v2"
	websocketserver "nhooyr.io/websocket"

	"github.com/iotaledger/hive.go/log"
	"github.com/iotaledger/hive.go/web/websockethub"
	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/publisher"
	"github.com/iotaledger/wasp/packages/webapi/websocket"
)

const (
	broadcastQueueSize            = 20000
	clientSendChannelSize         = 1000
	maxWebsocketMessageSize int64 = 510
)

func InitWebsocket(
	logger log.Logger,
	pub *publisher.Publisher,
	l1api iotago.API,
	maxTopicSubscriptionsPerClient int,
) (*websockethub.Hub, *websocket.Service) {
	websocketOptions := websocketserver.AcceptOptions{
		InsecureSkipVerify: true,
		// Disable compression due to incompatibilities with the latest Safari browsers:
		// https://github.com/tilt-dev/tilt/issues/4746
		CompressionMode: websocketserver.CompressionDisabled,
	}

	hub := websockethub.NewHub(logger, &websocketOptions, broadcastQueueSize, clientSendChannelSize, maxWebsocketMessageSize)

	websocketService := websocket.NewWebsocketService(logger, hub, []publisher.ISCEventType{
		publisher.ISCEventKindNewBlock,
		publisher.ISCEventKindReceipt,
		publisher.ISCEventIssuerVM,
		publisher.ISCEventKindBlockEvents,
	}, pub, l1api, websocket.WithMaxTopicSubscriptionsPerClient(maxTopicSubscriptionsPerClient))

	return hub, websocketService
}

func addWebSocketEndpoint(e echoswagger.ApiRoot, websocketPublisher *websocket.Service) {
	e.GET(fmt.Sprintf("/v%d/ws", APIVersion), websocketPublisher.ServeHTTP).
		SetSummary("The websocket connection service")
}
