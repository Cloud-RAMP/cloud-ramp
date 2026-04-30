package billing

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	gfirestore "cloud.google.com/go/firestore"
	"github.com/Cloud-RAMP/cloud-ramp.git/internal/cfg"
	"github.com/Cloud-RAMP/cloud-ramp.git/internal/firestore"
	"github.com/Cloud-RAMP/cloud-ramp.git/internal/logger"
)

// billing data saved for each instance
//
// using atomics here so that each operation does not require locking
type billingInfo struct {
	redisReads     atomic.Uint64
	redisWrites    atomic.Uint64
	redisPublishes atomic.Uint64

	firestoreReads  atomic.Uint64
	firestoreWrites atomic.Uint64

	firestoreWriteBytes atomic.Uint64
	firestoreReadBytes  atomic.Uint64

	outboundFetches atomic.Uint64

	inboundRequests atomic.Uint64
}

type billingMap struct {
	internalMap map[string]*billingInfo
	mu          sync.RWMutex
}

var billing billingMap

func init() {
	if !cfg.USE_FIRESTORE {
		return
	}

	billing.internalMap = make(map[string]*billingInfo)

	ticker := time.NewTicker(cfg.BILLING_DUMP_INTERVAL)
	go func() {
		for range ticker.C {
			logger.ServerInfo("Dumping billing data")
			err := OnBillingDump()
			if err != nil {
				logger.ServerError("Dumping billing info", err)
			}
		}
	}()
}

func OnBillingDump() error {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.BILLING_DUMP_INTERVAL)
	defer cancel()

	billing.mu.RLock()
	defer billing.mu.RUnlock()

	for id, b := range billing.internalMap {
		err := b.dumpSingleServicie(ctx, id)
		if err != nil {
			return err
		}
	}

	return nil
}

func (b *billingInfo) dumpSingleServicie(ctx context.Context, serviceId string) error {
	client, err := firestore.Client()
	if err != nil {
		return err
	}

	periodId := time.Now().UTC().Format("2006-01")
	ref := client.Collection("services").Doc(serviceId).
		Collection("billing").Doc(periodId)

	_, err = ref.Set(ctx, map[string]any{
		"redisReads":          gfirestore.Increment(int64(b.redisReads.Swap(0))),
		"redisWrites":         gfirestore.Increment(int64(b.redisWrites.Swap(0))),
		"firestoreReads":      gfirestore.Increment(int64(b.firestoreReads.Swap(0))),
		"firestoreWrites":     gfirestore.Increment(int64(b.firestoreWrites.Swap(0))),
		"firestoreWriteBytes": gfirestore.Increment(int64(b.firestoreWriteBytes.Swap(0))),
		"firestoreReadBytes":  gfirestore.Increment(int64(b.firestoreReadBytes.Swap(0))),
		"outboundFetches":     gfirestore.Increment(int64(b.outboundFetches.Swap(0))),
		"inboundRequests":     gfirestore.Increment(int64(b.inboundRequests.Swap(0))),
		"updatedAt":           time.Now().UTC(),
	}, gfirestore.MergeAll)

	return err
}
