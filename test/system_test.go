package system_test

import (
	"context"
	"encoding/csv"
	"fmt"
	"math/rand"
	"os"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
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
	"golang.org/x/time/rate"
)

// const TESTING_BASE_URL = "wss://cloud-ramp-578278759386.us-central1.run.app"
const TESTING_BASE_URL = "ws://localhost:8080"
const TESTING_MODULE_ID = "rP2gIxhkw7xHVpwGOX6g"
const ONLINE = true
const WARMUP = true

type sample struct {
	ts      int64
	elapsed int64
}

type result struct {
	targetRPS int
	actualRPS float64
	p50       int64
	p95       int64
	p99       int64
}

func TestMain(m *testing.M) {
	cfg.USE_FIRESTORE = false
	cfg.RATE_LIMIT = false

	parentCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if !ONLINE {
		if err := setup(parentCtx); err != nil {
			fmt.Println("Setup failed:", err)
			os.Exit(1)
		}

		time.Sleep(500 * time.Millisecond)
	}

	if WARMUP {
		url := fmt.Sprintf("%s/%s/a", TESTING_BASE_URL, TESTING_MODULE_ID)
		conn, _, _, err := ws.Dialer{}.Dial(context.Background(), url)
		if err != nil {
			fmt.Println("Failed to send warmup")
			return
		}

		for range 10 {
			wsutil.WriteClientMessage(conn, ws.OpText, []byte("warmup"))
			wsutil.ReadServerMessage(conn, nil)
		}

		conn.Close()
		time.Sleep(100 * time.Millisecond)
	}

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

func TestLatencyOverTime(t *testing.T) {
	duration := 30 * time.Second
	url := fmt.Sprintf("%s/%s/a", TESTING_BASE_URL, TESTING_MODULE_ID)
	conn, _, _, err := ws.Dialer{}.Dial(context.Background(), url)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	var samples []sample
	var MESSAGE = []byte("hello, websockets!")
	firstTimestamp := time.Now().UnixMilli()

	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	for ctx.Err() == nil {
		start := time.Now()

		err := wsutil.WriteClientMessage(conn, ws.OpText, MESSAGE)
		if err != nil {
			t.Fatalf("Failed write: %v", err)
		}

		_, err = wsutil.ReadServerMessage(conn, nil)
		if err != nil {
			t.Fatalf("Failed read: %v", err)
		}

		samples = append(samples, sample{
			ts:      start.UnixMilli() - firstTimestamp,
			elapsed: time.Since(start).Milliseconds(),
		})
	}

	writeCSV(t, samples, "results/latency_results.csv")
}

func writeCSV(t *testing.T, samples []sample, filename string) {
	t.Helper()

	f, err := os.Create(filename)
	if err != nil {
		t.Fatalf("Failed to create CSV: %v", err)
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

func TestLatencyVsThroughput(t *testing.T) {
	numConnections := 10
	rpsLevels := []int{5000, 10000, 15000, 20000, 25000, 30000, 32000, 34000, 36000}

	var results []result
	duration := 10 * time.Second

	for _, targetRPS := range rpsLevels {
		t.Run(fmt.Sprintf("target_rps=%d", targetRPS), func(t *testing.T) {
			var (
				mu      sync.Mutex
				samples []int64
				total   atomic.Int64
			)

			var limiter *rate.Limiter
			if targetRPS > 0 {
				limiter = rate.NewLimiter(rate.Limit(targetRPS), numConnections)
			}

			ctx, cancel := context.WithTimeout(context.Background(), duration)
			defer cancel()

			var wg sync.WaitGroup
			for range numConnections {
				wg.Add(1)
				go func() {
					defer wg.Done()

					url := fmt.Sprintf("%s/%s/a", TESTING_BASE_URL, TESTING_MODULE_ID)
					conn, _, _, err := ws.Dialer{}.Dial(context.Background(), url)
					if err != nil {
						t.Errorf("Failed to connect: %v", err)
						return
					}

					defer func() {
						time.Sleep(time.Duration(rand.Intn(200)) * time.Millisecond)
						wsutil.WriteClientMessage(conn, ws.OpClose, ws.NewCloseFrameBody(ws.StatusNormalClosure, ""))
						conn.Close()
					}()

					var MESSAGE = []byte("hello, websockets!")
					for {
						if ctx.Err() != nil {
							return
						}
						if limiter != nil {
							limiter.Wait(ctx)
						}

						start := time.Now()
						err := wsutil.WriteClientMessage(conn, ws.OpText, MESSAGE)
						if err != nil {
							t.Logf("rps=%d goroutine write error: %v", targetRPS, err)
							os.Exit(1)
						}
						_, err = wsutil.ReadServerMessage(conn, nil)
						if err != nil {
							t.Logf("rps=%d goroutine read error: %v", targetRPS, err)
							os.Exit(1)
						}
						elapsed := time.Since(start).Nanoseconds()

						mu.Lock()
						samples = append(samples, elapsed)
						mu.Unlock()
						total.Add(1)
					}
				}()
			}

			wg.Wait()

			time.Sleep(500 * time.Millisecond)

			slices.Sort(samples)
			actualRPS := float64(total.Load()) / duration.Seconds()

			results = append(results, result{
				targetRPS: targetRPS,
				actualRPS: actualRPS,
				p50:       percentile(samples, 0.50),
				p95:       percentile(samples, 0.95),
				p99:       percentile(samples, 0.99),
			})
		})
	}

	writeThroughputCSV(t, results)
}

func percentile(samples []int64, p float64) int64 {
	if len(samples) == 0 {
		return 0
	}
	idx := int(p * float64(len(samples)))
	if idx >= len(samples) {
		idx = len(samples) - 1
	}
	return samples[idx]
}

func writeThroughputCSV(t *testing.T, results []result) {
	t.Helper()
	f, err := os.Create("results/throughput_results.csv")
	if err != nil {
		t.Fatalf("Failed to create CSV: %v", err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	w.Write([]string{"target_rps", "actual_rps", "p50_ns", "p95_ns", "p99_ns"})
	for _, r := range results {
		w.Write([]string{
			strconv.Itoa(r.targetRPS),
			strconv.FormatFloat(r.actualRPS, 'f', 2, 64),
			strconv.FormatInt(r.p50, 10),
			strconv.FormatInt(r.p95, 10),
			strconv.FormatInt(r.p99, 10),
		})
	}
}
