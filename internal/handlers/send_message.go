package handlers

import (
	"context"
	"fmt"

	"github.com/Cloud-RAMP/cloud-ramp.git/internal/comm"
	"github.com/Cloud-RAMP/cloud-ramp.git/internal/redis"
	wasmevents "github.com/Cloud-RAMP/wasm-sandbox/pkg/wasm-events"
)

func SendMessageHandler(event *wasmevents.WASMEventInfo) (string, error) {
	if len(event.Payload) < 2 {
		return "", fmt.Errorf("Send message request missing required information")
	}

	// users exist on the same node, don't use redis
	if comm.UserOnSameNode(event.InstanceId, event.RoomId, event.Payload[0]) {
		return "", comm.SendEvent(&comm.CommEvent{
			Instance:  event.InstanceId,
			Room:      event.RoomId,
			DstConn:   event.Payload[0],
			SrcConn:   event.ConnectionId,
			EventType: comm.SEND_MESSAGE,
			Payload:   event.Payload[1],
		})
	}

	return "", redis.SendMessage(context.Background(),
		event.InstanceId,
		event.RoomId,
		event.ConnectionId,
		event.Payload[0],
		event.Payload[1],
	)
}
