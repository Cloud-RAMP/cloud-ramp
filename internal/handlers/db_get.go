package handlers

import (
	"context"
	"fmt"

	"github.com/Cloud-RAMP/cloud-ramp.git/internal/firestore"
	wasmevents "github.com/Cloud-RAMP/wasm-sandbox/pkg/wasm-events"
)

func DbGetHandler(event *wasmevents.WASMEventInfo) (string, error) {
	if len(event.Payload) < 1 {
		return "", fmt.Errorf("No get key provided")
	}

	return firestore.Get(
		context.Background(),
		event.InstanceId,
		event.RoomId,
		event.Payload[0],
	)
}
