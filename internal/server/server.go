package server

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"

	"github.com/google/uuid"
)

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

	// Here we can access info about the client, good for logging + billing purposes
	fmt.Println("New client connected:")
	fmt.Println("IP:", r.RemoteAddr)
	fmt.Println("Domain:", r.Host)

	id, err := uuid.NewV7()
	if err != nil {
		fmt.Println("Failed to create uuid")
		return
	}

	fmt.Println("ID is:", id)
	// PUT NEW USER ID IN KV-STORE

	parts := strings.Split(r.Host, ".")
	fmt.Println(parts)
	if len(parts) < 3 {
		fmt.Println("Invalid request domain")
		return
	}
	instanceId := parts[0]
	fmt.Println("InstanceID:", instanceId)

	url := r.URL
	room := url.Path
	fmt.Println("Room is:", room)

	// Start separate goroutine for each connection
	go func() {
		defer conn.Close() // when the function returns, close the connection

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
				return
			}

			// Probably not super necessary, good safeguard though
			// Other operation types CAN be sent
			if op.IsData() {
				fmt.Println("Client message:", string(msg))

				// TODO: do operations here
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
