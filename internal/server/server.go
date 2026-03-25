package server

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Cloud-RAMP/cloud-ramp.git/internal/sandbox"
	wsevents "github.com/Cloud-RAMP/wasm-sandbox/pkg/ws-events"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"

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

// Connection handler function
func handleConnection(w http.ResponseWriter, r *http.Request) {
	// Updagrade the HTTP connection to WS
	conn, _, _, err := ws.UpgradeHTTP(r, w)
	if err != nil {
		fmt.Println("Error upgrading protocols")
		return
	}

	// Create new unique client id
	id, err := uuid.NewV7()
	if err != nil {
		fmt.Println("Failed to create uuid")
		return
	}

	// TODO: PUT NEW USER ID IN KV-STORE

	// Split the host so that we can gather necessary info
	parts := strings.Split(r.Host, ".")
	if len(parts) < 3 {
		fmt.Println("Invalid request domain")
		return
	}
	instanceId := parts[0]
	url := r.URL
	room := url.Path

	baseEvent := wsevents.WSEventInfo{
		ConnectionId: id.String(),
		RoomId:       room,
		InstanceId:   instanceId,
	}

	// Start separate goroutine for each connection
	ctx, ctxClose := context.WithCancel(context.Background())

	// execute the initial on join event in a non-blocking goroutine
	go func() {
		event := baseEvent
		event.Timestamp = time.Now().UnixMilli()
		event.EventType = wsevents.ON_JOIN
		sandbox.Execute(ctx, &event)
	}()

	go func() {
		// the defers will run most-recent first, so the ON_LEAVE func will be 1st
		defer ctxClose()
		defer conn.Close()
		defer (func() {
			event := baseEvent
			event.Timestamp = time.Now().UnixMilli()
			event.EventType = wsevents.ON_LEAVE
			sandbox.Execute(ctx, &event)
		})()

		// infinite loop
		for {
			// Read data from the client on the connection
			// see https://datatracker.ietf.org/doc/html/rfc6455#section-5.5 for info on "op"
			msg, op, err := wsutil.ReadClientData(conn)
			if err == io.EOF {
				fmt.Println("Client disconnected")
				return
			} else if err != nil {
				// handle error
				fmt.Println("error reading:", err)

				// Can still just get an error on client disconnect, check that
				if err.Error() != LEAVE_ERROR {
					event := baseEvent
					event.Timestamp = time.Now().UnixMilli()
					event.EventType = wsevents.ON_ERROR
					event.Payload = err.Error()
					sandbox.Execute(ctx, &event)
					continue
				}

				// We hit here if the connection has closed, close the websocket
				return
			}

			if op.IsData() {
				event := baseEvent
				event.Timestamp = time.Now().UnixMilli()
				event.Payload = string(msg)
				event.EventType = wsevents.ON_MESSAGE

				// execute logs the error
				if sandbox.Execute(ctx, &event) != nil {
					continue
				}
			}

			// Write a message to the client as the server (in this case, echo it)
			err = wsutil.WriteServerMessage(conn, op, msg)
			if err == io.EOF {
				fmt.Println("Client disconnected")
				return
			} else if err != nil {
				fmt.Println("error reading:", err)
				return
			}
		}
	}()
}
