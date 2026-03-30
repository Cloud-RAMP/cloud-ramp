package logger

import (
	"fmt"
	"log/slog"
	"os"
)

// write to /tmp/cloudramp/log

var logFile *os.File
var logger *slog.Logger

func init() {
	const logDir = "/tmp/cloudramp"
	const logFilePath = logDir + "/log"

	err := os.MkdirAll(logDir, 0o755)
	if err != nil {
		fmt.Println("Could not create log directories", err)
		os.Exit(1)
	}

	logFile, err = os.OpenFile(logFilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Println("Could not open log file", err)
		os.Exit(1)
	}

	logger = slog.New(slog.NewTextHandler(logFile, nil))
}

func Info(instanceId, info string) {
	logger.Info(info, "instanceID", instanceId)
}

func Error(instanceId, info string) {
	logger.Error(info, "instanceID", instanceId)
}

func Warn(instanceId, info string) {
	logger.Warn(info, "instanceID", instanceId)
}
