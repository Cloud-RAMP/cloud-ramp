package handlers

import (
	wasmevents "github.com/Cloud-RAMP/wasm-sandbox/pkg/wasm-events"
)

func SetHandler(event *wasmevents.WASMEventInfo) (string, error) {
	if len(event.Payload) < 2 {
		return "", fmt.Errorf("Need to provide a key and value")
	}

	return "", redis.setDataValue(
		context.Background(), 
		event.InstanceId, 
		event.RoomId, 
		event.Payload[0],
		event.Payload[1]
	)
}
