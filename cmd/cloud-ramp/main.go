package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Cloud-RAMP/cloud-ramp.git/internal/cfg"
	"github.com/Cloud-RAMP/cloud-ramp.git/internal/firestore"
	"github.com/Cloud-RAMP/cloud-ramp.git/internal/handlers"
	"github.com/Cloud-RAMP/cloud-ramp.git/internal/logger"
	_ "github.com/Cloud-RAMP/cloud-ramp.git/internal/logger"
	"github.com/Cloud-RAMP/cloud-ramp.git/internal/redis"
	"github.com/Cloud-RAMP/cloud-ramp.git/internal/sandbox"
	"github.com/Cloud-RAMP/cloud-ramp.git/internal/server"
	"github.com/Cloud-RAMP/wasm-sandbox/pkg/store"
	wasmevents "github.com/Cloud-RAMP/wasm-sandbox/pkg/wasm-events"
	"github.com/joho/godotenv"
)

func main() {
	logger.ServerInfo("Loading environment variables")
	err := godotenv.Load()
	if err != nil {
		fmt.Println("No .env file found")
		os.Exit(1)
	}

	parentCtx, cancel := context.WithCancel(context.Background())

	logger.ServerInfo("Installing signal handler")
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		signal := <-c // wait for OS signal
		logger.ServerInfo(fmt.Sprintf("Program got signal: %s", signal))
		logger.ServerInfo("Beginning shutdown process")

		// stop server. the StartServer method of the server package has the remainder of the logic
		cancel()
	}()

	logger.ServerInfo("Initializing firestore")
	if cfg.USE_FIRESTORE {
		_, err = firestore.InitClient(parentCtx)
		if err != nil {
			logger.ServerError("Failed to initialize firestore:", err)
			os.Exit(1)
		}
	}

	logger.ServerInfo("Initializing sandbox")
	loader := sandbox.LoaderFunction
	if cfg.USE_MOCK_LOADER {
		loader = sandbox.DummyLoaderFunction
	}

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
		logger.ServerError("Failed to initialize sandbox:", err)
		os.Exit(1)
	}

	logger.ServerInfo("Connecting to redis")
	// Initialize redis client and start the server
	err = redis.InitClient(parentCtx)
	if err != nil {
		logger.ServerError("Failed to initialize redis client:", err)
		os.Exit(1)
	}

	logger.ServerInfo("Starting server")
	server.Start(parentCtx)
}
