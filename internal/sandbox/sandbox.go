package sandbox

import (
	"context"
	"fmt"

	"github.com/Cloud-RAMP/wasm-sandbox/pkg/store"
	wsevents "github.com/Cloud-RAMP/wasm-sandbox/pkg/ws-events"
)

var sandbox *store.SandboxStore

func InitializeSandbox(ctx context.Context, cfg store.SandboxStoreCfg) error {
	newSandbox, err := store.NewSandboxStore(ctx, cfg)
	if err != nil {
		sandbox = nil
		return err
	}

	sandbox = newSandbox
	return nil
}

func Execute(ctx context.Context, event *wsevents.WSEventInfo) error {
	err := sandbox.ExecuteOnModule(ctx, event)
	if err != nil {
		fmt.Println("Failed to execute properly:", err)
		return err
	}

	return nil
}
