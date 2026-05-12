package observability

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type eventStats struct {
	executions atomic.Uint64
	errors     atomic.Uint64
	durationMs atomic.Uint64
}

var (
	startedAt = time.Now()

	connectionsActive atomic.Int64
	connectionsTotal  atomic.Uint64
	connectionErrors  atomic.Uint64
	connectionRejects atomic.Uint64

	inboundMessages  atomic.Uint64
	outboundMessages atomic.Uint64
	outboundBytes    atomic.Uint64
	rateLimited      atomic.Uint64

	externalMessages      atomic.Uint64
	externalMessageErrors atomic.Uint64

	eventMu sync.RWMutex
	events  = make(map[string]*eventStats)
)

func ConnectionOpened() {
	connectionsTotal.Add(1)
	connectionsActive.Add(1)
}

func ConnectionClosed() {
	connectionsActive.Add(-1)
}

func ConnectionError() {
	connectionErrors.Add(1)
}

func ConnectionRejected() {
	connectionRejects.Add(1)
}

func InboundMessage() {
	inboundMessages.Add(1)
}

func OutboundMessage(bytes int) {
	outboundMessages.Add(1)
	if bytes > 0 {
		outboundBytes.Add(uint64(bytes))
	}
}

func RateLimited() {
	rateLimited.Add(1)
}

func ExternalMessage() {
	externalMessages.Add(1)
}

func ExternalMessageError() {
	externalMessageErrors.Add(1)
}

func SandboxExecution(eventType string, duration time.Duration, err error) {
	stats := statsForEvent(eventType)
	stats.executions.Add(1)
	stats.durationMs.Add(uint64(duration.Milliseconds()))
	if err != nil {
		stats.errors.Add(1)
	}
}

func MetricsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

	now := time.Now()
	lines := []string{
		"# HELP cloud_ramp_uptime_seconds Seconds since this process started.",
		"# TYPE cloud_ramp_uptime_seconds gauge",
		fmt.Sprintf("cloud_ramp_uptime_seconds %.0f", now.Sub(startedAt).Seconds()),
		"# HELP cloud_ramp_connections_active Currently open WebSocket connections.",
		"# TYPE cloud_ramp_connections_active gauge",
		fmt.Sprintf("cloud_ramp_connections_active %d", connectionsActive.Load()),
		"# HELP cloud_ramp_connections_total Total accepted WebSocket connections.",
		"# TYPE cloud_ramp_connections_total counter",
		fmt.Sprintf("cloud_ramp_connections_total %d", connectionsTotal.Load()),
		"# HELP cloud_ramp_connection_errors_total WebSocket connections closed through the error path.",
		"# TYPE cloud_ramp_connection_errors_total counter",
		fmt.Sprintf("cloud_ramp_connection_errors_total %d", connectionErrors.Load()),
		"# HELP cloud_ramp_connection_rejects_total Connection attempts rejected before WebSocket upgrade.",
		"# TYPE cloud_ramp_connection_rejects_total counter",
		fmt.Sprintf("cloud_ramp_connection_rejects_total %d", connectionRejects.Load()),
		"# HELP cloud_ramp_ws_inbound_messages_total WebSocket data messages received from clients.",
		"# TYPE cloud_ramp_ws_inbound_messages_total counter",
		fmt.Sprintf("cloud_ramp_ws_inbound_messages_total %d", inboundMessages.Load()),
		"# HELP cloud_ramp_ws_outbound_messages_total WebSocket messages sent to clients.",
		"# TYPE cloud_ramp_ws_outbound_messages_total counter",
		fmt.Sprintf("cloud_ramp_ws_outbound_messages_total %d", outboundMessages.Load()),
		"# HELP cloud_ramp_ws_outbound_bytes_total WebSocket payload bytes sent to clients.",
		"# TYPE cloud_ramp_ws_outbound_bytes_total counter",
		fmt.Sprintf("cloud_ramp_ws_outbound_bytes_total %d", outboundBytes.Load()),
		"# HELP cloud_ramp_rate_limited_total Requests denied by the rate limiter.",
		"# TYPE cloud_ramp_rate_limited_total counter",
		fmt.Sprintf("cloud_ramp_rate_limited_total %d", rateLimited.Load()),
		"# HELP cloud_ramp_external_messages_total Messages received from local or Redis channels for WebSocket delivery.",
		"# TYPE cloud_ramp_external_messages_total counter",
		fmt.Sprintf("cloud_ramp_external_messages_total %d", externalMessages.Load()),
		"# HELP cloud_ramp_external_message_errors_total External messages that could not be marshaled or written.",
		"# TYPE cloud_ramp_external_message_errors_total counter",
		fmt.Sprintf("cloud_ramp_external_message_errors_total %d", externalMessageErrors.Load()),
		"# HELP cloud_ramp_sandbox_executions_total WASM sandbox executions by WebSocket event type.",
		"# TYPE cloud_ramp_sandbox_executions_total counter",
	}

	eventMu.RLock()
	eventNames := make([]string, 0, len(events))
	for eventType := range events {
		eventNames = append(eventNames, eventType)
	}
	sort.Strings(eventNames)
	for _, eventType := range eventNames {
		stats := events[eventType]
		label := fmt.Sprintf(`event_type="%s"`, prometheusEscape(eventType))
		lines = append(lines,
			fmt.Sprintf("cloud_ramp_sandbox_executions_total{%s} %d", label, stats.executions.Load()),
			fmt.Sprintf("cloud_ramp_sandbox_errors_total{%s} %d", label, stats.errors.Load()),
			fmt.Sprintf("cloud_ramp_sandbox_duration_ms_sum{%s} %d", label, stats.durationMs.Load()),
			fmt.Sprintf("cloud_ramp_sandbox_duration_ms_count{%s} %d", label, stats.executions.Load()),
		)
	}
	eventMu.RUnlock()

	_, _ = fmt.Fprintln(w, strings.Join(lines, "\n"))
}

func statsForEvent(eventType string) *eventStats {
	if eventType == "" {
		eventType = "unknown"
	}

	eventMu.RLock()
	stats, ok := events[eventType]
	eventMu.RUnlock()
	if ok {
		return stats
	}

	eventMu.Lock()
	defer eventMu.Unlock()
	if stats, ok = events[eventType]; ok {
		return stats
	}
	stats = &eventStats{}
	events[eventType] = stats
	return stats
}

func prometheusEscape(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, "\n", `\n`)
	return strings.ReplaceAll(value, `"`, `\"`)
}
