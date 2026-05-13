package cfg

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/Cloud-RAMP/cloud-ramp.git/internal/logger"
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
		logger.ServerError("reding config post request", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	configReq := configRequest{}
	err = json.Unmarshal(reqBytes, &configReq)
	if err != nil {
		logger.ServerError("unmarshalling config post json", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if configReq.LogLevel != nil {
		LOG_LEVEL.Store(*configReq.LogLevel)
		logger.ServerInfo(fmt.Sprintf("Updated log level: %d", *configReq.LogLevel))
	}

	if configReq.RateLimit != nil {
		RATE_LIMIT.Store(*configReq.RateLimit)
		logger.ServerInfo(fmt.Sprintf("Updated rate limit bool: %v", *configReq.RateLimit))
	}

	if configReq.MaxRequestsPerWindow != nil {
		MAX_REQUESTS_PER_WINDOW.Store(*configReq.MaxRequestsPerWindow)
		logger.ServerInfo(fmt.Sprintf("Updated max requests per window: %d", *configReq.MaxRequestsPerWindow))
	}

	w.WriteHeader(http.StatusOK)
}

func handleConfigGet(w http.ResponseWriter, r *http.Request) {
	configBytes, err := json.Marshal(getConfig())

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		logger.ServerError("marshalling config get json", err)
		return
	}

	logger.ServerInfo("Config get request", slog.Attr{
		Key:   "ip",
		Value: slog.StringValue(r.RemoteAddr),
	})

	w.Write(configBytes)
}
