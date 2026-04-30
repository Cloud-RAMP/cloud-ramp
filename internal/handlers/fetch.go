package handlers

import (
	"bytes"
	"fmt"
	"io"
	"net/http"

	"github.com/Cloud-RAMP/cloud-ramp.git/internal/billing"
	"github.com/Cloud-RAMP/cloud-ramp.git/internal/logger"
	wasmevents "github.com/Cloud-RAMP/wasm-sandbox/pkg/wasm-events"
)

func FetchHandler(event *wasmevents.WASMEventInfo) (string, error) {
	if len(event.Payload) < 2 {
		return "", fmt.Errorf("Request is missing URL or HTTP method")
	}
	logger.WASMEvent(event)

	url := event.Payload[0]
	method := event.Payload[1]
	body := event.Payload[2]

	req, err := http.NewRequest(method, url, bytes.NewBufferString(body))
	if err != nil {
		return "", err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	billing.OutboundFetch(event.InstanceId, uint64(len(body)))
	return string(respBody), nil
}
