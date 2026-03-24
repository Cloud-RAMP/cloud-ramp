package handlers

import (
	"fmt"

	wasmevents "github.com/Cloud-RAMP/wasm-sandbox/pkg/wasm-events"
)

func DebugHandler(event *wasmevents.WASMEventInfo) (string, error) {
	if len(event.Payload) > 0 {
		fmt.Println("User called debug:", event.Payload[0])
	}

	return "dummy", nil
}
