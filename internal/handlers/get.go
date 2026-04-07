package handlers

import (
	wasmevents "github.com/Cloud-RAMP/wasm-sandbox/pkg/wasm-events"
)

func GetHandler(event *wasmevents.WASMEventInfo) (string, error) {
	if len(event.Payload) < 1 {
		return "", fmt.Errorf("Need to provide a key")
	}

	return "", redis.setDataValue(
		context.Background(), 
		event.InstanceId, 
		event.RoomId, 
		event.Payload[0],
		event.Payload[1]
	)
}

