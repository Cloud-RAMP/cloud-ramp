package handlers

import (
	"context"
	"fmt"

	"github.com/Cloud-RAMP/cloud-ramp.git/internal/redis"
	wasmevents "github.com/Cloud-RAMP/wasm-sandbox/pkg/wasm-events"
)

func BroadcastHandler(event *wasmevents.WASMEventInfo) (string, error) {
	if len(event.Payload) < 1 {
		return "", fmt.Errorf("No broadcast message specified")
	}

	return "", redis.Broadcast(
		context.Background(),
		event.InstanceId,
		event.RoomId,
		event.ConnectionId,
		event.Payload[0],
	)
}
