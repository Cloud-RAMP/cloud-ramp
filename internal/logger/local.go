package logger

import (
	"context"
	"log/slog"
)

// This file deals with all things local logging
//
// "Local logging" refers to logging operations on the server,
// printing messages to STDOUT that we will be able to see from
// whatever cloud platform we deploy on

// Log an info message to the server
func ServerInfo(msg string, attrs ...slog.Attr) {
	slog.LogAttrs(context.Background(), slog.LevelInfo, msg, attrs...)
}

// Log an error message to the server
func ServerError(msg string, err error, attrs ...slog.Attr) {
	if err != nil {
		attrs = append([]slog.Attr{
			{
				Key:   "errMsg",
				Value: slog.StringValue(err.Error()),
			}}, attrs...)
		slog.LogAttrs(context.Background(), slog.LevelError, msg, attrs...)
	} else {
		slog.LogAttrs(context.Background(), slog.LevelError, msg, attrs...)
	}
}
