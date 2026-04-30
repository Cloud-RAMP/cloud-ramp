package logger

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"

	wasmevents "github.com/Cloud-RAMP/wasm-sandbox/pkg/wasm-events"
	wsevents "github.com/Cloud-RAMP/wasm-sandbox/pkg/ws-events"
)

// write to /tmp/cloudramp/logs/{instanceID}.log
const logsDir = "/tmp/cloudramp/logs"

var mu sync.Mutex
var loggers map[string]*slog.Logger

func init() {
	err := os.MkdirAll(logsDir, 0o755)
	if err != nil {
		fmt.Println("Could not create log directories", err)
		os.Exit(1)
	}

	loggers = make(map[string]*slog.Logger)
}

func getLogger(instanceId string) *slog.Logger {
	mu.Lock()
	defer mu.Unlock()

	if l, ok := loggers[instanceId]; ok {
		return l
	}

	logFilePath := fmt.Sprintf("%s/%s.log", logsDir, instanceId)
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Println("Could not open log file", err)
		return slog.New(slog.NewJSONHandler(os.Stderr, nil))
	}

	logger := slog.New(slog.NewJSONHandler(logFile, nil))
	loggers[instanceId] = logger

	return logger
}

func RemoveLogger(instanceId string) error {
	mu.Lock()
	defer mu.Unlock()

	delete(loggers, instanceId)

	logFilePath := fmt.Sprintf("%s/%s.log", logsDir, instanceId)
	if err := os.Remove(logFilePath); err != nil {
		return err
	}

	return nil
}

func Warn(instanceId, connectionId, info string) {
	getLogger(instanceId).Warn(info, "instanceID", instanceId, "connectionID", connectionId)
}

// Log a WASM event
func WASMEvent(event *wasmevents.WASMEventInfo) {
	if event == nil {
		Warn("unknown", "unknown", "WASMEvent called with nil event")
		return
	}

	getLogger(event.InstanceId).Info("WASMEvent",
		"connectionID",
		event.ConnectionId,
		"roomID",
		event.RoomId,
		"eventType",
		event.EventType.String(),
		"payload",
		strings.Join(event.Payload, ","),
	)
}

// Log a ws event
func WSEvent(event *wsevents.WSEventInfo) {
	if event == nil {
		Warn("unknown", "unknown", "WSEvent called with nil event")
		return
	}

	getLogger(event.InstanceId).Info("WSEvent",
		"connectionID",
		event.ConnectionId,
		"roomID",
		event.RoomId,
		"eventType",
		event.EventType.String(),
		"payload",
		event.Payload,
	)
}

// Log a new connection
func NewConnection(instanceId, ip, connectionId, roomId string) {
	getLogger(instanceId).Info("NewConnection",
		"instanceId",
		instanceId,
		"roomId",
		roomId,
		"connectionId",
		connectionId,
		"ip",
		ip,
	)
}

func Info(instanceId string, msg string, attrs ...slog.Attr) {
	getLogger(instanceId).LogAttrs(context.Background(), slog.LevelInfo, msg, attrs...)
}

func Error(instanceId string, msg string, attrs ...slog.Attr) {
	getLogger(instanceId).LogAttrs(context.Background(), slog.LevelError, msg, attrs...)
}
