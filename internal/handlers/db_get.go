package handlers

import (
	"context"
	"fmt"

	"github.com/Cloud-RAMP/cloud-ramp.git/internal/billing"
	"github.com/Cloud-RAMP/cloud-ramp.git/internal/firestore"
	"github.com/Cloud-RAMP/cloud-ramp.git/internal/logger"
	wasmevents "github.com/Cloud-RAMP/wasm-sandbox/pkg/wasm-events"
)

func DbGetHandler(event *wasmevents.WASMEventInfo) (string, error) {
	if len(event.Payload) < 1 {
		return "", fmt.Errorf("No get key provided")
	}
	logger.WASMEvent(event)

	res, err := firestore.Get(
		context.Background(),
		event.InstanceId,
		event.RoomId,
		event.Payload[0],
	)
	billing.FirestoreRead(event.InstanceId, uint64(len(res)))
	return res, err
}
