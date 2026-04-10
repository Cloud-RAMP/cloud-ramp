package handlers

import (
	"context"
	"fmt"

	"github.com/Cloud-RAMP/cloud-ramp.git/internal/comm"
	"github.com/Cloud-RAMP/cloud-ramp.git/internal/logger"
	"github.com/Cloud-RAMP/cloud-ramp.git/internal/redis"
	wasmevents "github.com/Cloud-RAMP/wasm-sandbox/pkg/wasm-events"
)

func CloseConnectionHandler(event *wasmevents.WASMEventInfo) (string, error) {
	if len(event.Payload) < 1 {
		return "", fmt.Errorf("No target connection provided")
	}
	logger.WASMEvent(event)

	commEvent := comm.CommEvent{
		DstConn:   event.Payload[0],
		SrcConn:   event.ConnectionId,
		EventType: comm.CLOSE_CONNECTION,
		Instance:  event.InstanceId,
		Room:      event.RoomId,
	}

	return "", redis.SendCommEvent(context.Background(), &commEvent)
}
