package system_test

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/Cloud-RAMP/cloud-ramp.git/internal/cfg"
	"github.com/Cloud-RAMP/cloud-ramp.git/internal/handlers"
	"github.com/Cloud-RAMP/cloud-ramp.git/internal/redis"
	"github.com/Cloud-RAMP/cloud-ramp.git/internal/sandbox"
	"github.com/Cloud-RAMP/cloud-ramp.git/internal/server"
	"github.com/Cloud-RAMP/wasm-sandbox/pkg/store"
	wasmevents "github.com/Cloud-RAMP/wasm-sandbox/pkg/wasm-events"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/joho/godotenv"
)

const TESTING_BASE_URL = "ws://localhost:8080"
const TESTING_MODULE_ID = "rP2gIxhkw7xHVpwGOX6g"

type sample struct {
	ts      int64
	elapsed int64
}

func TestMain(m *testing.M) {
	cfg.USE_FIRESTORE = false
	cfg.RATE_LIMIT = false

	parentCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := setup(parentCtx); err != nil {
		fmt.Println("Setup failed:", err)
		os.Exit(1)
	}

	time.Sleep(500 * time.Millisecond)

	os.Exit(m.Run())
}

func setup(ctx context.Context) error {
	godotenv.Load("../.env")

	err := sandbox.InitializeSandbox(ctx, store.SandboxStoreCfg{
		CleanupInterval:    cfg.MODULE_CLEANUP_INTERVAL,
		MaxIdleTime:        cfg.MAX_MODULE_IDLE_TIME,
		MemoryLimitPages:   10,
		MaxActiveModules:   10,
		PoolSize:           5,
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
			AddHandler(wasmevents.SERVER_MESSAGE, handlers.ServerMessageHandler).
			AddHandler(wasmevents.CLOSE_CONNECTION, handlers.CloseConnectionHandler).
			AddHandler(wasmevents.FETCH, handlers.FetchHandler),
		LoaderFunction: sandbox.LoaderFunction,
	})
	if err != nil {
		fmt.Println("Failed to setup sandbox")
		return err
	}

	err = redis.InitClient(ctx)
	if err != nil {
		fmt.Println("Failed to setup redis")
		return err
	}

	go server.Start(ctx)
	return nil
}

func BenchmarkLatencyOverTime(b *testing.B) {
	url := fmt.Sprintf("%s/%s/a", TESTING_BASE_URL, TESTING_MODULE_ID)
	conn, _, _, err := ws.Dialer{}.Dial(context.Background(), url)
	if err != nil {
		b.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	var samples []sample

	var MESSAGE = []byte("hello, websockets!")
	for b.Loop() {
		start := time.Now()

		err := wsutil.WriteClientMessage(conn, ws.OpText, MESSAGE)
		if err != nil {
			b.Fatalf("Failed write: %e", err)
		}

		_, err = wsutil.ReadServerMessage(conn, nil)
		if err != nil {
			b.Fatalf("Failed read message: %e", err)
		}

		samples = append(samples, sample{
			ts:      start.UnixMilli(),
			elapsed: time.Since(start).Nanoseconds(),
		})
	}

	writeCSV(b, samples)
}

func writeCSV(b *testing.B, samples []sample) {
	b.Helper()

	f, err := os.Create("results/latency_results.csv")
	if err != nil {
		b.Fatalf("Failed to create CSV: %v", err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	w.Write([]string{"timestamp_ms", "latency_ms"})
	for _, s := range samples {
		w.Write([]string{
			strconv.FormatInt(s.ts, 10),
			strconv.FormatInt(s.elapsed, 10),
		})
	}
}
