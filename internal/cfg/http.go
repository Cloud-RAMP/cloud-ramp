package cfg

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
)

type configRequest struct {
	LogLevel             *uint32 `json:"log_level"`
	RateLimit            *bool   `json:"rate_limit"`
	MaxRequestsPerWindow *uint32 `json:"max_requests_per_window"`

	// change timeouts / intervals for things too?
}

type configResponse struct {
	// Editable
	LogLevel             uint32 `json:"log_level"`
	RateLimit            bool   `json:"rate_limit"`
	MaxRequestsPerWindow uint32 `json:"max_requests_per_window"`
	MaxModuleIdleTime    int    `json:"max_module_idle_time_seconds"`
	MsgJoinLeave         bool   `json:"msg_join_leave"`

	// Read-only / informational
	Env                    int  `json:"env"`
	UseFirestore           bool `json:"use_firestore"`
	RateLimitWindowSeconds int  `json:"rate_limit_window_seconds"`
}

func getConfig() configResponse {
	return configResponse{
		LogLevel:             LOG_LEVEL.Load(),
		RateLimit:            RATE_LIMIT.Load(),
		MaxRequestsPerWindow: MAX_REQUESTS_PER_WINDOW.Load(),
		MaxModuleIdleTime:    MAX_MODULE_IDLE_TIME_SECONDS,
		MsgJoinLeave:         MSG_JOIN_LEAVE,

		Env:                    ENV,
		UseFirestore:           USE_FIRESTORE,
		RateLimitWindowSeconds: RATE_LIMIT_WINDOW_SECONDS,
	}
}

func HandleConfigRequest(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		handleConfigPost(w, r)
	case http.MethodGet:
		handleConfigGet(w, r)
	default:
		w.WriteHeader(http.StatusBadRequest)
	}
}

func handleConfigPost(w http.ResponseWriter, r *http.Request) {
	reqBytes, err := io.ReadAll(r.Body)
	if err != nil {
		logError("reding config post request", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	configReq := configRequest{}
	err = json.Unmarshal(reqBytes, &configReq)
	if err != nil {
		logError("unmarshalling config post json", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if configReq.LogLevel != nil {
		LOG_LEVEL.Store(*configReq.LogLevel)
		logInfo(fmt.Sprintf("Updated log level: %d", *configReq.LogLevel))
	}

	if configReq.RateLimit != nil {
		RATE_LIMIT.Store(*configReq.RateLimit)
		logInfo(fmt.Sprintf("Updated rate limit bool: %v", *configReq.RateLimit))
	}

	if configReq.MaxRequestsPerWindow != nil {
		MAX_REQUESTS_PER_WINDOW.Store(*configReq.MaxRequestsPerWindow)
		logInfo(fmt.Sprintf("Updated max requests per window: %d", *configReq.MaxRequestsPerWindow))
	}

	w.WriteHeader(http.StatusOK)
}

func handleConfigGet(w http.ResponseWriter, r *http.Request) {
	configBytes, err := json.Marshal(getConfig())

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		logError("marshalling config get json", err)
		return
	}

	logInfo("Config get request", slog.Attr{
		Key:   "ip",
		Value: slog.StringValue(r.RemoteAddr),
	})

	w.Write(configBytes)
}

// copy of server info log
func logInfo(msg string, attrs ...slog.Attr) {
	slog.LogAttrs(context.Background(), slog.LevelInfo, msg, attrs...)
}

// copy of server info error
func logError(msg string, err error, attrs ...slog.Attr) {
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
