package handlers

import (
	wasmevents "github.com/Cloud-RAMP/wasm-sandbox/pkg/wasm-events"
)

func FetchHandler(event *wasmevents.WASMEventInfo) (string, error) {
	return "dummy", nil
}
