package handlers

import (
	wasmevents "github.com/Cloud-RAMP/wasm-sandbox/pkg/wasm-events"
)

func GetHandler(event *wasmevents.WASMEventInfo) (string, error) {
	return "dummy", nil
}
