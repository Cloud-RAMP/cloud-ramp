package handlers

import (
	"context"
	"fmt"

	"github.com/Cloud-RAMP/cloud-ramp.git/internal/billing"
	"github.com/Cloud-RAMP/cloud-ramp.git/internal/logger"
	"github.com/Cloud-RAMP/cloud-ramp.git/internal/redis"
	wasmevents "github.com/Cloud-RAMP/wasm-sandbox/pkg/wasm-events"
)

func GetHandler(event *wasmevents.WASMEventInfo) (string, error) {
	if len(event.Payload) < 1 {
		return "", fmt.Errorf("Need to provide a key")
	}
	logger.WASMEvent(event)

	billing.RedisRead(event.InstanceId)

	return redis.GetDataValue(
		context.Background(),
		event.InstanceId,
		event.RoomId,
		event.Payload[0],
	)
}
