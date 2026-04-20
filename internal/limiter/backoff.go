package limiter

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/Cloud-RAMP/cloud-ramp.git/internal/cfg"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
)

func BackoffHTTP(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(429)
	fmt.Fprintf(w, "Rate limit exceeded. Try again in %d seconds", cfg.RATE_LIMIT_WINDOW_SECONDS)
}

func BackoffWS(conn net.Conn) error {
	rateLimitExceeded := struct {
		Status    string `json:"status"`
		Message   string `json:"message"`
		Timestamp string `json:"timestamp"`
	}{
		Status:    "error",
		Message:   fmt.Sprintf("Rate limit exceeded. Try again in %d seconds", cfg.RATE_LIMIT_WINDOW_SECONDS),
		Timestamp: time.Now().Format(time.RFC3339),
	}

	// won't fail, we control all of the inputs
	jsonBytes, err := json.Marshal(rateLimitExceeded)
	if err != nil {
		return err
	}

	return wsutil.WriteServerMessage(conn, ws.OpText, jsonBytes)
}
