package cfg

import "net/http"

type configRequest struct {
	LogLevel  uint8 `json:"log_level"`
	RateLimit bool  `json:"rate_limit"`

	// change timeouts / intervals for things too?
}

func HandleConfigRequest(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		handleConfigPost(w, r)
	case http.MethodGet:
		handleConfigGet(w, r)
	default:
		w.WriteHeader(400)
	}
}

func handleConfigPost(w http.ResponseWriter, r *http.Request) {

}

func handleConfigGet(w http.ResponseWriter, r *http.Request) {

}
