package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/Cloud-RAMP/cloud-ramp.git/internal/cfg"
	"github.com/Cloud-RAMP/cloud-ramp.git/internal/firestore"
	"github.com/Cloud-RAMP/cloud-ramp.git/internal/handlers"
	_ "github.com/Cloud-RAMP/cloud-ramp.git/internal/logger"
	"github.com/Cloud-RAMP/cloud-ramp.git/internal/redis"
	"github.com/Cloud-RAMP/cloud-ramp.git/internal/sandbox"
	"github.com/Cloud-RAMP/cloud-ramp.git/internal/server"
	"github.com/Cloud-RAMP/wasm-sandbox/pkg/store"
	wasmevents "github.com/Cloud-RAMP/wasm-sandbox/pkg/wasm-events"
	"github.com/joho/godotenv"
)

func main() {
	fmt.Println("Loading environment variables")
	err := godotenv.Load()
	if err != nil {
		fmt.Println("No .env file found")
		os.Exit(1)
	}

	parentCtx := context.Background()

	fmt.Println("Initializing firestore")
	if cfg.USE_FIRESTORE {
		_, err = firestore.InitClient(parentCtx)
		if err != nil {
			fmt.Println("Failed to initialize firestore:", err)
			os.Exit(1)
		}
	}

	loader := sandbox.LoaderFunction
	if cfg.USE_MOCK_LOADER {
		loader = sandbox.DummyLoaderFunction
	}

	fmt.Println("Initializing sandbox")
	// These values will probably need to be changed later to ones that make sense for the system
	err = sandbox.InitializeSandbox(parentCtx, store.SandboxStoreCfg{
		CleanupInterval:    5 * time.Second,
		MaxIdleTime:        6 * time.Second,
		MemoryLimitPages:   10,
		MaxActiveModules:   3,
		CloseOnContextDone: true,
		HandlerMap: wasmevents.NewHandlerMap().
			AddHandler(wasmevents.ABORT, handlers.AbortHandler).
			AddHandler(wasmevents.GET, handlers.GetHandler).
			AddHandler(wasmevents.SET, handlers.SetHandler).
			AddHandler(wasmevents.DEL, handlers.DelHandler).
			AddHandler(wasmevents.DB_GET, handlers.DbGetHandler).
			AddHandler(wasmevents.DB_SET, handlers.DbSetHandler).
			AddHandler(wasmevents.DB_DEL, handlers.DbDelHandler).
			AddHandler(wasmevents.BROADCAST, handlers.BroadcastHandler).
			AddHandler(wasmevents.LOG, handlers.LogHandler).
			AddHandler(wasmevents.DEBUG, handlers.DebugHandler).
			AddHandler(wasmevents.GET_USERS, handlers.GetUsersHandler).
			AddHandler(wasmevents.SEND_MESSAGE, handlers.SendMessageHandler).
			AddHandler(wasmevents.CLOSE_CONNECTION, handlers.CloseConnectionHandler).
			AddHandler(wasmevents.FETCH, handlers.FetchHandler),
		LoaderFunction: loader,
	})
	if err != nil {
		fmt.Println("Failed to initialize sandbox:", err)
		os.Exit(1)
	}

	fmt.Println("Connecting to redis")
	// Initialize redis client and start the server
	err = redis.InitClient(parentCtx)
	if err != nil {
		fmt.Println("Failed to initialize redis client:", err)
		os.Exit(1)
	}

	fmt.Println("Starting server")
	server.Start(parentCtx)
}
