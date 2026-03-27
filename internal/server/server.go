package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/Cloud-RAMP/cloud-ramp.git/internal/comm"
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
) {
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
				fmt.Println("Failed to marshal local communication json:", err)
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
				fmt.Println("Failed to marshal json:", err)
				continue
			}

			// this message is not meant for us, or we sent it
			if (event.DstConn != "*" && event.DstConn != connId) || event.SrcConn == connId {
				continue
			}

			err = wsutil.WriteServerMessage(conn, ws.OpText, []byte(redisEvent.Payload))
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

	// Split the host so that we can gather necessary info
	parts := strings.Split(r.Host, ".")
	if len(parts) < 3 {
		fmt.Println("Invalid request domain")
		return
	}
	instanceId := parts[0]
	url := r.URL
	room := strings.TrimPrefix(url.Path, "/")

	baseEvent := wsevents.WSEventInfo{
		ConnectionId: connId.String(),
		RoomId:       room,
		InstanceId:   instanceId,
	}

	// Background context for goroutine
	ctx, ctxClose := context.WithCancel(context.Background())

	// execute the initial on join event
	event := baseEvent
	event.Timestamp = time.Now().UnixMilli()
	event.EventType = wsevents.ON_JOIN
	sandbox.Execute(ctx, &event)

	commChan := comm.InitConn(instanceId, room, connId.String())
	redisChan, err := redis.JoinRoom(ctx, instanceId, room, connId.String())
	if err != nil {
		fmt.Printf("Connection %s failed to join redis room: %v\n", connId, err)
		ctxClose()
		return
	}

	go handleExternalMessages(ctx, conn, commChan, redisChan, connId.String())
	go func() {
		// the defers will run most-recent first, so the ON_LEAVE func will be 1st
		defer conn.Close()
		defer comm.CloseConn(instanceId, room, connId.String())
		defer ctxClose()
		defer redis.LeaveRoom(ctx, instanceId, room, connId.String())
		defer (func() {
			event := baseEvent
			event.Timestamp = time.Now().UnixMilli()
			event.EventType = wsevents.ON_LEAVE
			sandbox.Execute(ctx, &event)
		})()

		// loop for duration of the connection
		for {
			// Read data from the client on the connection
			// see https://datatracker.ietf.org/doc/html/rfc6455#section-5.5 for info on "op"
			msg, op, err := wsutil.ReadClientData(conn)
			if err == io.EOF {
				fmt.Println("Client disconnected")
				return
			} else if err != nil {
				// Client disconnected
				if err.Error() == LEAVE_ERROR {
					return
				}

				fmt.Println("Error reading:", err)
				event := baseEvent
				event.Timestamp = time.Now().UnixMilli()
				event.EventType = wsevents.ON_ERROR
				event.Payload = err.Error()
				sandbox.Execute(ctx, &event)
			}

			if op.IsData() {
				event := baseEvent
				event.Timestamp = time.Now().UnixMilli()
				event.Payload = string(msg)
				event.EventType = wsevents.ON_MESSAGE

				// execute logs any errors
				sandbox.Execute(ctx, &event)
			}

			// Write a message to the client as the server (in this case, echo it)
			err = wsutil.WriteServerMessage(conn, op, msg)
			if err == io.EOF {
				fmt.Println("Client disconnected")
				return
			} else if err != nil {
				if err.Error() == LEAVE_ERROR {
					return
				}

				fmt.Println("Error writing:", err)
				event := baseEvent
				event.Timestamp = time.Now().UnixMilli()
				event.EventType = wsevents.ON_ERROR
				event.Payload = err.Error()
				sandbox.Execute(ctx, &event)
			}
		}
	}()
}
