package handlers

import (
	"github.com/Cloud-RAMP/cloud-ramp.git/internal/logger"
	wasmevents "github.com/Cloud-RAMP/wasm-sandbox/pkg/wasm-events"
)

func LogHandler(event *wasmevents.WASMEventInfo) (string, error) {
	logger.WASMEvent(event)
	return "", nil
}
