package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Cloud-RAMP/cloud-ramp.git/internal/billing"
	"github.com/Cloud-RAMP/cloud-ramp.git/internal/cfg"
	"github.com/Cloud-RAMP/cloud-ramp.git/internal/comm"
	"github.com/Cloud-RAMP/cloud-ramp.git/internal/firestore"
	"github.com/Cloud-RAMP/cloud-ramp.git/internal/limiter"
	"github.com/Cloud-RAMP/cloud-ramp.git/internal/logger"
	"github.com/Cloud-RAMP/cloud-ramp.git/internal/observability"
	"github.com/Cloud-RAMP/cloud-ramp.git/internal/redis"
	"github.com/Cloud-RAMP/cloud-ramp.git/internal/sandbox"
	wsevents "github.com/Cloud-RAMP/wasm-sandbox/pkg/ws-events"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"

	_redis "github.com/redis/go-redis/v9"

	"github.com/google/uuid"
)

const WS_NO_STATUS = "ws closed: 1005 "
const WS_GOING_AWAY = "ws closed: 1001 "

// Define start method here so that we can use it in testing
//
// It's a good habit to use these "ctx" objects, since they give us fine grained
// control over server state
func Start(ctx context.Context) {
	server := &http.Server{
		Addr:    ":8080",
		Handler: http.HandlerFunc(routeRequest),
	}

	go func() {
		logger.ServerInfo("Server listening on :8080")
		err := server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			logger.ServerError("Starting server", err)
			return
		}
	}()

	// On shutdown, give the server a 15s timer to complete all connections
	// Then dump logs and rate limits
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	server.Shutdown(shutdownCtx)

	if cfg.USE_FIRESTORE {
		logger.ServerInfo("Dumping logs")
		err := firestore.OnLogDump()
		if err != nil {
			logger.ServerError("Dumping logs in shutdown", err)
		}
	}

	if cfg.USE_FIRESTORE {
		logger.ServerInfo("Dumping billing data")
		err := billing.OnBillingDump()
		if err != nil {
			logger.ServerError("Dumping billing data in shutdown", err)
		}
	}

	logger.ServerInfo("Dumping limiter")
	err := limiter.OnDump()
	if err != nil {
		logger.ServerError("Dumping limiter in shutdown", err)
	}

	logger.ServerInfo("Shutdown complete")
}

func routeRequest(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/healthz":
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	case "/readyz":
		handleReadiness(w, r)
	case "/metrics":
		observability.MetricsHandler(w, r)
	default:
		handleConnection(w, r)
	}
}

func handleReadiness(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	checks := map[string]string{
		"redis":     "ok",
		"sandbox":   "ok",
		"firestore": "disabled",
	}
	status := http.StatusOK

	if err := redis.Ping(ctx); err != nil {
		checks["redis"] = err.Error()
		status = http.StatusServiceUnavailable
	}

	if !sandbox.IsInitialized() {
		checks["sandbox"] = "not initialized"
		status = http.StatusServiceUnavailable
	}

	if cfg.USE_FIRESTORE {
		checks["firestore"] = "ok"
		if _, err := firestore.Client(); err != nil {
			checks["firestore"] = err.Error()
			status = http.StatusServiceUnavailable
		}
	}

	writeJSON(w, status, map[string]any{
		"status": statusText(status),
		"checks": checks,
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		logger.ServerError("Encoding JSON response", err)
	}
}

func statusText(status int) string {
	if status >= 200 && status < 300 {
		return "ok"
	}
	return "unavailable"
}

func executeSandbox(ctx context.Context, event *wsevents.WSEventInfo) error {
	start := time.Now()
	err := sandbox.Execute(ctx, event)

	eventType := "unknown"
	if event != nil {
		eventType = event.EventType.String()
	}
	observability.SandboxExecution(eventType, time.Since(start), err)
	return err
}

func SendFailure(w http.ResponseWriter, r *http.Request) {
	observability.ConnectionRejected()
	w.WriteHeader(500)
	fmt.Fprintf(w, "Failed to establish WebSocket connection")
}

// Connection handler function
func handleConnection(w http.ResponseWriter, r *http.Request) {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		logger.ServerError("gathering IP address:", err)
		SendFailure(w, r)
		return
	}

	backoff, err := limiter.RegisterNewConnection(ip)
	if err != nil {
		logger.ServerError("register new rate limiter connection:", err)
		SendFailure(w, r)
		return
	}
	if backoff {
		observability.RateLimited()
		observability.ConnectionRejected()
		limiter.BackoffHTTP(w, r)
		return
	}

	// Split path so we can gather necessary info (instanceId, room)
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/"), "/")
	if len(parts) < 2 {
		logger.ServerError("Invalid request domain:", err)
		observability.ConnectionRejected()
		return
	}
	instanceId := parts[0]
	room := parts[1]

	// Background context
	ctx, ctxClose := context.WithCancel(context.Background())

	// fetch connection ID if this IP has connected before
	connId, err := redis.CheckUserID(ctx, instanceId, room, ip)
	if err != nil {
		logger.ServerError("connection UID recovery:", err)
		SendFailure(w, r)
		ctxClose()
		return
	}
	if connId == "" {
		// Create new unique client id
		connIdUuid, err := uuid.NewV7()
		if err != nil {
			logger.ServerError("Failed to create uuid:", err)
			observability.ConnectionRejected()
			ctxClose()
			return
		}
		connId = connIdUuid.String()
	}

	// construct a base event that we will modify to send to WebAssembly
	baseEvent := wsevents.WSEventInfo{
		ConnectionId: connId,
		RoomId:       room,
		InstanceId:   instanceId,
	}

	logger.NewConnection(instanceId, r.RemoteAddr, connId, room)

	// establish room chan and join redis pub/sub
	commChan := comm.InitConn(instanceId, room, connId)
	redisChan, err := redis.JoinRoom(ctx, instanceId, room, connId)
	if err != nil {
		logger.Error(instanceId, fmt.Sprintf("Connection %s failed to join redis room: %v", connId, err))
		observability.ConnectionRejected()
		ctxClose()
		return
	}

	// Updagrade the HTTP connection to WS once bookkeeping is done
	conn, _, _, err := ws.UpgradeHTTP(r, w)
	if err != nil {
		logger.ServerError("upgrading protocols:", err)
		SendFailure(w, r)
		ctxClose()
		return
	}
	observability.ConnectionOpened()

	// execute the initial on join event
	event := baseEvent
	event.Timestamp = time.Now().UnixMilli()
	event.EventType = wsevents.ON_JOIN
	if err = executeSandbox(ctx, &event); err != nil {
		logger.Error(instanceId, fmt.Sprintf("Failed to execute onJoin event: %v", err), slog.Attr{
			Key:   "connectionId",
			Value: slog.StringValue(connId),
		}, slog.Attr{
			Key:   "roomId",
			Value: slog.StringValue(room),
		})
	}

	// only execute cleanup code once
	var oneClose sync.Once
	var oneError sync.Once
	var closed atomic.Bool

	onConnectionClose := func() {
		if closed.Swap(true) {
			return
		}

		oneClose.Do(func() {
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cleanupCancel()

			logger.Info(instanceId, "Client disconnected", slog.Attr{
				Key:   "connectionId",
				Value: slog.StringValue(connId),
			}, slog.Attr{
				Key:   "roomId",
				Value: slog.StringValue(room),
			})

			event := baseEvent
			event.Timestamp = time.Now().UnixMilli()
			event.EventType = wsevents.ON_LEAVE
			if err := executeSandbox(cleanupCtx, &event); err != nil {
				logger.ServerError("Failed to execute on close", err)
			}

			limiter.DumpConnectionRequests(ip)
			err := redis.LeaveRoom(cleanupCtx, instanceId, room, connId, ip)
			if err != nil {
				slog.Error("Failed redis leave", "errMsg", err.Error())
			}
			ctxClose()
			comm.CloseConn(instanceId, room, connId)
			conn.Close()
			observability.ConnectionClosed()
		})
	}

	onConnectionError := func(err error) {
		if closed.Swap(true) {
			return
		}

		oneError.Do(func() {
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cleanupCancel()

			logger.ServerError("WebSocket write failed", err)
			logger.Info(instanceId, fmt.Sprintf("Client error writing: %v", err), slog.Attr{
				Key:   "connectionId",
				Value: slog.StringValue(connId),
			}, slog.Attr{
				Key:   "roomId",
				Value: slog.StringValue(room),
			})

			event := baseEvent
			event.Timestamp = time.Now().UnixMilli()
			event.EventType = wsevents.ON_ERROR
			event.Payload = err.Error()
			if err := executeSandbox(cleanupCtx, &event); err != nil {
				logger.ServerError("Failed to execute onError event", err)
			}

			limiter.DumpConnectionRequests(ip)
			redis.LeaveRoom(cleanupCtx, instanceId, room, connId, ip)
			ctxClose()
			comm.CloseConn(instanceId, room, connId)
			conn.Close()
			observability.ConnectionError()
			observability.ConnectionClosed()
		})
	}

	// Spin off two goroutines: one for receiving messages, one for sending
	go handleExternalMessages(ctx, conn, commChan, redisChan, connId, instanceId, onConnectionClose)
	func() {
		// loop for duration of the connection
		for {
			select {
			case <-ctx.Done():
				return
			default:
				// Read data from the client on the connection
				// see https://datatracker.ietf.org/doc/html/rfc6455#section-5.5 for info on "op"
				msg, op, err := wsutil.ReadClientData(conn)
				if err == io.EOF {
					onConnectionClose()
					return
				} else if err != nil {
					// Client disconnected
					if err.Error() == WS_NO_STATUS || err.Error() == WS_GOING_AWAY || ctx.Err() != nil {
						onConnectionClose()
						return
					}

					onConnectionError(err)
					return
				}

				billing.InboundRequest(instanceId)

				// Check rate limiter, do we need to backoff?
				backoff := limiter.RegisterNewRequest(ip)
				if backoff {
					observability.RateLimited()
					if err := limiter.BackoffWS(conn); err != nil {
						onConnectionError(err)
						return
					}

					logger.Info(instanceId, "Rate limit", slog.Attr{
						Key:   "connectionId",
						Value: slog.StringValue(connId),
					}, slog.Attr{
						Key:   "IP",
						Value: slog.StringValue(ip),
					}, slog.Attr{
						Key:   "roomId",
						Value: slog.StringValue(room),
					}, slog.Attr{
						Key:   "payload",
						Value: slog.StringValue(string(msg)),
					})

					continue
				}

				// user sent actual data
				if op.IsData() {
					observability.InboundMessage()
					event := baseEvent
					event.Timestamp = time.Now().UnixMilli()
					event.Payload = string(msg)
					event.EventType = wsevents.ON_MESSAGE

					logger.Info(instanceId, "New message", slog.Attr{
						Key:   "connectionId",
						Value: slog.StringValue(connId),
					}, slog.Attr{
						Key:   "roomId",
						Value: slog.StringValue(room),
					}, slog.Attr{
						Key:   "payload",
						Value: slog.StringValue(string(msg)),
					})

					// execute logs any errors
					if err = executeSandbox(ctx, &event); err != nil {
						logger.ServerError("Failed to execute onMessage event", err)
					}
				}
			}
		}
	}()
}

// Helper function to be detached in a separate goroutine and handle connections from outside sources
//
// Messages can come from either redis or the same server
func handleExternalMessages(
	ctx context.Context,
	conn net.Conn,
	commChan <-chan *comm.CommEvent,
	redisChan <-chan *_redis.Message,
	connId string,
	instanceId string,
	onConnectionClose func(),
) {
	defer onConnectionClose()

	for {
		select {
		case <-ctx.Done():
			return
		case commEvent, ok := <-commChan:
			if !ok {
				return
			}
			observability.ExternalMessage()

			// Send data to the connection
			json, err := json.Marshal(commEvent)
			if err != nil {
				observability.ExternalMessageError()
				logger.Error(instanceId, fmt.Sprintf("Failed to marshal local communication json: %v", err), slog.Attr{
					Key:   "connectionId",
					Value: slog.StringValue(connId),
				})
				continue
			}

			billing.OutboundBytes(instanceId, uint64(len(json)))
			if err = wsutil.WriteServerMessage(conn, ws.OpText, json); err != nil {
				observability.ExternalMessageError()
				return
			}
			observability.OutboundMessage(len(json))
		case redisEvent, ok := <-redisChan:
			if !ok {
				// channel closed
				return
			}
			observability.ExternalMessage()

			event := &comm.CommEvent{}
			err := json.Unmarshal([]byte(redisEvent.Payload), event)
			if err != nil {
				observability.ExternalMessageError()
				logger.Error(instanceId, fmt.Sprintf("Failed to marshal redis json: %v", err), slog.Attr{
					Key:   "connectionId",
					Value: slog.StringValue(connId),
				})
				continue
			}

			// this message is not meant for us, or we sent it
			if (event.DstConn != "*" && event.DstConn != connId) || event.SrcConn == connId {
				continue
			}

			billing.OutboundBytes(instanceId, uint64(len(redisEvent.Payload)))
			if err = wsutil.WriteServerMessage(conn, ws.OpText, []byte(redisEvent.Payload)); err != nil {
				observability.ExternalMessageError()
				return
			}
			observability.OutboundMessage(len(redisEvent.Payload))

			// connection is closed. returns to that the onConnectionClose function will run
			if event.EventType == comm.CLOSE_CONNECTION {
				return
			}
		}
	}
}
