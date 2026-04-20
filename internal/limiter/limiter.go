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
	"time"

	"github.com/Cloud-RAMP/cloud-ramp.git/internal/cfg"
	"github.com/Cloud-RAMP/cloud-ramp.git/internal/redis"
)

type RateMapEntry struct {
	NumRequests int
	Connected   bool
	LastRequest time.Time
	WindowStart time.Time
}

var rateMap map[string]RateMapEntry = make(map[string]RateMapEntry)

// Initialize service to dump connection information to redis on timeout
func init() {
	ticker := time.NewTicker(cfg.RATE_DUMP)
	go func() {
		for range ticker.C {
			OnDump()
		}
	}()
}

// Dump all current rate information redis
//
// Also clear out any disconnections
func OnDump() {
	lockAll()
	for k, entry := range rateMap {
		// wipe users who aren't connected and TTL has expired
		if !entry.Connected && time.Since(entry.WindowStart) >= cfg.RATE_TTL {
			delete(rateMap, k)
			continue
		}

		// TTL of redis will be the default TTL - time since the window started
		ttl := cfg.RATE_TTL - time.Since(entry.WindowStart)

		// only send our data to redis if TTL is non-negative
		if ttl > 0 {
			if err := redis.SetCurrentRequests(k, entry.NumRequests, ttl); err != nil {
				fmt.Println("SERVER ERROR: setting current requests in redis")
			}
		}
	}

	unlockAll()
}

// To be called when a new connection joins
//
// Fetch existing rate limiting data from redis, backoff if necessary
func RegisterNewConnection(ip string) (bool, error) {
	locks.lock(ip)
	currentRequests, err := redis.GetCurrentRequests(ip)
	if err != nil {
		return false, err
	}

	currentRequests++

	entry, ok := rateMap[ip]
	if !ok {
		rateMap[ip] = RateMapEntry{
			NumRequests: currentRequests,
			LastRequest: time.Now(),
			WindowStart: time.Now(),
			Connected:   true,
		}
	} else {
		entry.Connected = true
		entry.NumRequests = currentRequests
		rateMap[ip] = entry
	}

	locks.unlock(ip)

	if currentRequests > cfg.MAX_REQUESTS_PER_WINDOW {
		return true, nil
	}

	return false, nil
}

// Call this function on every new request to see if a backoff is necessary
//
// If so, call one of the limiter.Backoff functions (depending on if you are HTTP or WS connection)
//
// The user will be rate limited if:
//   - They have exceeded the max number of reqeusts AND
//   - The time since their window started is less than the max
//
// The window will be reset if:
//   - The time since the start of their window is >= than the max
//   - If the window is reset, their number of requests sent is reset as well
func RegisterNewRequest(ip string) bool {
	locks.lock(ip)
	mapEntry := rateMap[ip]

	defer func() {
		mapEntry.NumRequests++
		mapEntry.LastRequest = time.Now()
		rateMap[ip] = mapEntry
		locks.unlock(ip)
	}()

	if time.Since(mapEntry.WindowStart) >= cfg.RATE_TTL {
		mapEntry.WindowStart = time.Now()
		mapEntry.NumRequests = 0
		return false
	}

	if mapEntry.NumRequests >= cfg.MAX_REQUESTS_PER_WINDOW && time.Since(mapEntry.WindowStart) <= cfg.RATE_TTL {
		return true
	}

	return false
}

// To be called when a client disconnects
//
// Dump their rate info to redis
func DumpConnectionRequests(ip string) error {
	locks.lock(ip)
	entry, ok := rateMap[ip]

	if !ok {
		locks.unlock(ip)
		return nil
	}

	entry.Connected = false
	if time.Since(entry.WindowStart) >= cfg.RATE_TTL { // window has expired, if they reconnect they can have new session
		delete(rateMap, ip)
		locks.unlock(ip)
		return nil
	} else {
		rateMap[ip] = entry
	}

	ttl := cfg.RATE_TTL - time.Since(entry.WindowStart)
	locks.unlock(ip)
	return redis.SetCurrentRequests(ip, entry.NumRequests, ttl)
}
