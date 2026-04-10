package handlers

import (
	"context"
	"fmt"

	"github.com/Cloud-RAMP/cloud-ramp.git/internal/redis"
	wasmevents "github.com/Cloud-RAMP/wasm-sandbox/pkg/wasm-events"
)

func SetHandler(event *wasmevents.WASMEventInfo) (string, error) {
	if len(event.Payload) < 2 {
		return "", fmt.Errorf("Need to provide a key and value")
	}

	return "", redis.SetDataValue(
		context.Background(),
		event.InstanceId,
		event.RoomId,
		event.Payload[0],
		event.Payload[1],
	)
}
