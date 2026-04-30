package handlers

import (
	"context"
	"fmt"

	"github.com/Cloud-RAMP/cloud-ramp.git/internal/billing"
	"github.com/Cloud-RAMP/cloud-ramp.git/internal/firestore"
	"github.com/Cloud-RAMP/cloud-ramp.git/internal/logger"
	wasmevents "github.com/Cloud-RAMP/wasm-sandbox/pkg/wasm-events"
)

func DbSetHandler(event *wasmevents.WASMEventInfo) (string, error) {
	if len(event.Payload) < 2 {
		return "", fmt.Errorf("Invalid number of arguments returned")
	}
	logger.WASMEvent(event)

	billing.FirestoreWrite(event.InstanceId, uint64(len(event.Payload[1])))

	return "", firestore.Set(
		context.Background(),
		event.InstanceId,
		event.RoomId,
		event.Payload[0],
		event.Payload[1],
	)
}
