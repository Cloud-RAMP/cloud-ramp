package main

import (
	"context"
	"time"

	"github.com/Cloud-RAMP/cloud-ramp.git/internal/handlers"
	"github.com/Cloud-RAMP/cloud-ramp.git/internal/sandbox"
	"github.com/Cloud-RAMP/cloud-ramp.git/internal/server"
	"github.com/Cloud-RAMP/wasm-sandbox/pkg/store"
	wasmevents "github.com/Cloud-RAMP/wasm-sandbox/pkg/wasm-events"
)

func main() {
	parentCtx := context.Background()

	// These values will probably need to be changed later to ones that make sense for the system
	sandbox.InitializeSandbox(parentCtx, store.SandboxStoreCfg{
		CleanupInterval:    5 * time.Second,
		MaxIdleTime:        6 * time.Second,
		MemoryLimitPages:   10,
		MaxActiveModules:   3,
		CloseOnContextDone: true,
		HandlerMap: wasmevents.NewHandlerMap().
			AddHandler(wasmevents.ABORT, handlers.AbortHandler).
			AddHandler(wasmevents.GET, handlers.GetHandler).
			AddHandler(wasmevents.SET, handlers.SetHandler).
			AddHandler(wasmevents.DB_GET, handlers.DbGetHandler).
			AddHandler(wasmevents.DB_SET, handlers.DbSetHandler).
			AddHandler(wasmevents.BROADCAST, handlers.BroadcastHandler).
			AddHandler(wasmevents.LOG, handlers.LogHandler).
			AddHandler(wasmevents.DEBUG, handlers.DebugHandler).
			AddHandler(wasmevents.GET_USERS, handlers.GetUsersHandler).
			AddHandler(wasmevents.SEND_MESSAGE, handlers.SendMessageHandler).
			AddHandler(wasmevents.FETCH, handlers.FetchHandler),
		// LoaderFunction: sandbox.LoaderFunction,
		LoaderFunction: sandbox.DummyLoaderFunction,
	})

	server.Start(parentCtx)
}
