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
	"time"

	"github.com/Cloud-RAMP/cloud-ramp.git/internal/comm"
	"github.com/Cloud-RAMP/cloud-ramp.git/internal/logger"
	"github.com/Cloud-RAMP/cloud-ramp.git/internal/redis"
	"github.com/Cloud-RAMP/cloud-ramp.git/internal/sandbox"
	wsevents "github.com/Cloud-RAMP/wasm-sandbox/pkg/ws-events"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"

	_redis "github.com/redis/go-redis/v9"

	"github.com/google/uuid"
)

const LEAVE_ERROR = "ws closed: 1005 "

// Define start method here so that we can use it in testing
//
// It's a good habit to use these "ctx" objects, since they give us fine grained
// control over server state
func Start(ctx context.Context) {
	server := &http.Server{
		Addr:    ":8080",
		Handler: http.HandlerFunc(handleConnection),
	}

	go func() {
		fmt.Println("Server listening on", server.Addr)
		err := server.ListenAndServe()
		if err != nil {
			fmt.Println("Error starting server", err)
			return
		}
	}()

	// This is "channel" syntax, basically it means that we wait until something is sent in the channel
	// Channels can be used to send data, but in this case it is used as a signal by sending an empty struct
	<-ctx.Done()
	server.Shutdown(ctx)
}

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

			// Send data to the connection
			json, err := json.Marshal(commEvent)
			if err != nil {
				logger.Error(instanceId, fmt.Sprintf("Failed to marshal local communication json: %e", err), slog.Attr{
					Key:   "connectionId",
					Value: slog.StringValue(connId),
				})
				continue
			}

			err = wsutil.WriteServerMessage(conn, ws.OpText, json)
		case redisEvent, ok := <-redisChan:
			if !ok {
				// channel closed
				return
			}

			event := &comm.CommEvent{}
			err := json.Unmarshal([]byte(redisEvent.Payload), event)
			if err != nil {
				logger.Error(instanceId, fmt.Sprintf("Failed to marshal redis json: %e", err), slog.Attr{
					Key:   "connectionId",
					Value: slog.StringValue(connId),
				})
				continue
			}

			// this message is not meant for us, or we sent it
			if (event.DstConn != "*" && event.DstConn != connId) || event.SrcConn == connId {
				continue
			}

			err = wsutil.WriteServerMessage(conn, ws.OpText, []byte(redisEvent.Payload))

			// connection is closed. returns to that the onConnectionClose function will run
			if event.EventType == comm.CLOSE_CONNECTION {
				return
			}
		}
	}
}

// Connection handler function
func handleConnection(w http.ResponseWriter, r *http.Request) {
	// Updagrade the HTTP connection to WS
	conn, _, _, err := ws.UpgradeHTTP(r, w)
	if err != nil {
		fmt.Println("Error upgrading protocols")
		return
	}

	// Create new unique client id
	connId, err := uuid.NewV7()
	if err != nil {
		fmt.Println("Failed to create uuid")
		return
	}

	// Split path so we can gather necessary info
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/"), "/")
	if len(parts) < 2 {
		fmt.Println("Invalid request domain")
		return
	}
	instanceId := parts[0]
	room := parts[1]

	baseEvent := wsevents.WSEventInfo{
		ConnectionId: connId.String(),
		RoomId:       room,
		InstanceId:   instanceId,
	}

	logger.NewConnection(instanceId, r.RemoteAddr, connId.String(), room)

	// Background context for goroutine
	ctx, ctxClose := context.WithCancel(context.Background())

	// establish room chan and join redis pub/sub
	commChan := comm.InitConn(instanceId, room, connId.String())
	redisChan, err := redis.JoinRoom(ctx, instanceId, room, connId.String())
	if err != nil {
		fmt.Printf("Connection %s failed to join redis room: %v\n", connId, err)
		logger.Error(instanceId, fmt.Sprintf("Connection %s failed to join redis room: %v", connId, err))
		ctxClose()
		return
	}

	// execute the initial on join event
	event := baseEvent
	event.Timestamp = time.Now().UnixMilli()
	event.EventType = wsevents.ON_JOIN
	if err = sandbox.Execute(ctx, &event); err != nil {
		logger.Error(instanceId, fmt.Sprintf("Failed to execute onJoin event: %e", err), slog.Attr{
			Key:   "connectionId",
			Value: slog.StringValue(connId.String()),
		}, slog.Attr{
			Key:   "roomId",
			Value: slog.StringValue(room),
		})
	}

	// only execute cleanup code once
	var once sync.Once
	onConnectionClose := func() {
		once.Do(func() {
			logger.Info(instanceId, "Client disconnected", slog.Attr{
				Key:   "connectionId",
				Value: slog.StringValue(connId.String()),
			}, slog.Attr{
				Key:   "roomId",
				Value: slog.StringValue(room),
			})

			event := baseEvent
			event.Timestamp = time.Now().UnixMilli()
			event.EventType = wsevents.ON_LEAVE
			sandbox.Execute(ctx, &event)
			redis.LeaveRoom(ctx, instanceId, room, connId.String())
			ctxClose()
			comm.CloseConn(instanceId, room, connId.String())
			conn.Close()
		})
	}

	onConnectionError := func(err error) {
		fmt.Println("Error writing:", err)
		logger.Info(instanceId, fmt.Sprintf("Client error writing: %e", err), slog.Attr{
			Key:   "connectionId",
			Value: slog.StringValue(connId.String()),
		}, slog.Attr{
			Key:   "roomId",
			Value: slog.StringValue(room),
		})
		event := baseEvent
		event.Timestamp = time.Now().UnixMilli()
		event.EventType = wsevents.ON_ERROR
		event.Payload = err.Error()
		sandbox.Execute(ctx, &event)
		redis.LeaveRoom(ctx, instanceId, room, connId.String())
		ctxClose()
		comm.CloseConn(instanceId, room, connId.String())
		conn.Close()
	}

	go handleExternalMessages(ctx, conn, commChan, redisChan, connId.String(), instanceId, onConnectionClose)
	go func() {
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
					fmt.Println("Client disconnected")
					onConnectionClose()
					return
				} else if err != nil {
					// Client disconnected
					if err.Error() == LEAVE_ERROR || ctx.Err() != nil {
						onConnectionClose()
						return
					}

					onConnectionError(err)
					return
				}

				if op.IsData() {
					event := baseEvent
					event.Timestamp = time.Now().UnixMilli()
					event.Payload = string(msg)
					event.EventType = wsevents.ON_MESSAGE

					logger.Info(instanceId, "New message", slog.Attr{
						Key:   "connectionId",
						Value: slog.StringValue(connId.String()),
					}, slog.Attr{
						Key:   "roomId",
						Value: slog.StringValue(room),
					}, slog.Attr{
						Key:   "payload",
						Value: slog.StringValue(string(msg)),
					})

					// execute logs any errors
					if err = sandbox.Execute(ctx, &event); err != nil {
						logger.Error(instanceId, fmt.Sprintf("Failed to execute onJoin event: %e", err), slog.Attr{
							Key:   "connectionId",
							Value: slog.StringValue(connId.String()),
						}, slog.Attr{
							Key:   "roomId",
							Value: slog.StringValue(room),
						})
					}
				}

				// Write a message to the client as the server (in this case, echo it)
				err = wsutil.WriteServerMessage(conn, op, msg)
				if err == io.EOF {
					fmt.Println("Client disconnected")
					onConnectionClose()
					return
				} else if err != nil {
					if err.Error() == LEAVE_ERROR || ctx.Err() != nil {
						onConnectionClose()
						return
					}

					onConnectionError(err)
					return
				}
			}
		}
	}()
}
