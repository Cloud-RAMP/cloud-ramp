module github.com/Cloud-RAMP/cloud-ramp.git

go 1.24.1

require (
	github.com/Cloud-RAMP/wasm-sandbox v0.0.0-20260311195328-fb246bdb4f88
	github.com/gobwas/ws v1.4.0
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/joho/godotenv v1.5.1 // indirect
	github.com/redis/go-redis/v9 v9.18.0 // indirect
	github.com/tetratelabs/wazero v1.11.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/multierr v1.10.0 // indirect
	go.uber.org/zap v1.27.1 // indirect
)

require (
	github.com/gobwas/httphead v0.1.0 // indirect
	github.com/gobwas/pool v0.2.1 // indirect
	github.com/google/uuid v1.6.0
	golang.org/x/sys v0.38.0 // indirect
)

replace github.com/Cloud-RAMP/wasm-sandbox => ../wasm-sandbox
