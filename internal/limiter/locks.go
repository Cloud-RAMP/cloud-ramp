package limiter

import (
	"hash/crc32"
	"sync"
)

type lockMap map[uint32]*sync.Mutex

const num_locks = 27

var locks lockMap = make(lockMap)
var globalMu sync.RWMutex

func init() {
	var i uint32
	for i = range num_locks {
		locks[i] = &sync.Mutex{}
	}
}

func hash(ip string) uint32 {
	return crc32.ChecksumIEEE([]byte(ip)) % num_locks
}

func (lm *lockMap) lock(ip string) {
	globalMu.RLock()
	locks[hash(ip)].Lock()
}

func (lm *lockMap) unlock(ip string) {
	locks[hash(ip)].Unlock()
	globalMu.RUnlock()
}

// other operations cannot proceed if global mu is locked
func lockAll() {
	globalMu.Lock()
}

func unlockAll() {
	globalMu.Unlock()
}
