// package limiter
//
// # Provides rate limiting functions for the main server
//
// Every function in this package will return a boolean (and maybe an error)
// indicating if the user should be backed off or not
//
// The architecture is as follows:
//   - upon new connection, a user will be registered.
//     This means grabbing current rate information from redis,
//     and adding them to the server-map that keeps track of rates
//   - For every new WS request, we will check locally their rate.
//     If they exceed what is defined in cfg, they will be backed off
//   - It could be a good idea for users to specify a rate for their app.
//     Not implemented yet, but future idea for more extensibility
package limiter

import (
	"fmt"
	"sync"
	"time"

	"github.com/Cloud-RAMP/cloud-ramp.git/internal/cfg"
	"github.com/Cloud-RAMP/cloud-ramp.git/internal/redis"
)

type RateMapEntry struct {
	NumRequests int
	LastRequest time.Time
}

var rateMap map[string]RateMapEntry = make(map[string]RateMapEntry)
var mu sync.Mutex = sync.Mutex{}

// Initialize service to dump connection information to redis
func init() {
	ticker := time.NewTicker(cfg.RATE_DUMP)
	go func() {
		for range ticker.C {
			handleRedisDump()
		}
	}()
}

// Dump current information on connections to redis
func handleRedisDump() {
	fmt.Println("Dumping rate info to redis")

	mu.Lock()
	var wg sync.WaitGroup

	for k, v := range rateMap {
		sinceLastRequest := time.Since(v.LastRequest)
		ttl := min(cfg.RATE_TTL, sinceLastRequest)

		wg.Add(1)
		go func() {
			fmt.Printf("%s requests: %d\n", k, v.NumRequests)
			if err := redis.SetCurrentRequests(k, v.NumRequests, ttl); err != nil {
				fmt.Println("SERVER ERROR: setting current requests in redis")
			}
			wg.Done()
		}()
	}

	wg.Wait()
	mu.Unlock()
}

func RegisterNewConnection(ip string) (bool, error) {
	currentRequests, err := redis.GetCurrentRequests(ip)
	if err != nil {
		return false, err
	}

	currentRequests++

	mu.Lock()
	rateMap[ip] = RateMapEntry{
		NumRequests: currentRequests,
		LastRequest: time.Now(),
	}
	mu.Unlock()

	if currentRequests > cfg.MAX_REQUESTS_PER_WINDOW {
		return true, nil
	}

	return false, nil
}

// Call this function on every new request to see if a backoff is necessary
//
// If so, call one of the limiter.Backoff functions (depending on if you are HTTP or WS connection)
func RegisterNewRequest(ip string) bool {
	mu.Lock()
	mapEntry := rateMap[ip]
	mu.Unlock()

	mapEntry.NumRequests++

	defer func() {
		mapEntry.LastRequest = time.Now()
		mu.Lock()
		rateMap[ip] = mapEntry
		mu.Unlock()
	}()

	if mapEntry.NumRequests >= cfg.MAX_REQUESTS_PER_WINDOW && time.Since(mapEntry.LastRequest) <= cfg.RATE_TTL {
		return true
	}

	return false
}

// To be called when a client disconnects
//
// Dump their rate info to redis
func DumpConnectionRequests(ip string) error {
	mu.Lock()
	info, ok := rateMap[ip]
	mu.Unlock()

	if !ok {
		return nil
	}

	sinceLastRequest := time.Since(info.LastRequest)
	ttl := min(cfg.RATE_TTL, sinceLastRequest)
	return redis.SetCurrentRequests(ip, info.NumRequests, ttl)
}
