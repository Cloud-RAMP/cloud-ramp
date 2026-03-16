package comm

import (
	"fmt"
	"maps"
	"sync"
)

type CommEventType int

const (
	SEND_MESSAGE CommEventType = iota
	BROADCAST
	LEAVE
	JOIN
)

type CommEvent struct {
	destInstance string
	destRoom     string
	destConn     string
	srcInstance  string
	srcRoom      string
	srcConn      string
	payload      string
	eventType    CommEventType
}

type CommRoom struct {
	conns map[string]chan *CommEvent
	mu    sync.Mutex
}

const CHAN_SIZE = 100

// commMap has compound key "instance:room"
var commMap map[string]*CommRoom
var mu sync.Mutex

func init() {
	commMap = make(map[string]*CommRoom)
}

// getRoomKey creates a compound key from instance and room
func getRoomKey(instanceId, roomId string) string {
	return fmt.Sprintf("%s:%s", instanceId, roomId)
}

func SendEvent(e *CommEvent) error {
	roomKey := getRoomKey(e.destInstance, e.destRoom)

	mu.Lock()
	room, ok := commMap[roomKey]
	mu.Unlock()

	if !ok {
		return fmt.Errorf("invalid destination instance/room: %s/%s", e.destInstance, e.destRoom)
	}

	room.mu.Lock()
	userChan, ok := room.conns[e.destConn]
	room.mu.Unlock()

	if !ok {
		return fmt.Errorf("invalid destination connection: %s", e.destConn)
	}

	// non-blocking, avoid deadlock
	select {
	case userChan <- e:
		return nil
	default:
		return fmt.Errorf("channel full or unavailable for connection: %s", e.destConn)
	}
}

func InitConn(instanceId, roomId, connId string) {
	roomKey := getRoomKey(instanceId, roomId)

	mu.Lock()
	room, ok := commMap[roomKey]
	if !ok {
		room = &CommRoom{
			conns: make(map[string]chan *CommEvent),
		}
		commMap[roomKey] = room
	}
	mu.Unlock()

	room.mu.Lock()
	defer room.mu.Unlock()

	if _, ok := room.conns[connId]; !ok {
		room.conns[connId] = make(chan *CommEvent, CHAN_SIZE)
	}
}

func CloseConn(instanceId, roomId, connId string) {
	roomKey := getRoomKey(instanceId, roomId)

	mu.Lock()
	room, ok := commMap[roomKey]
	mu.Unlock()

	if !ok {
		return
	}

	room.mu.Lock()
	defer func() {
		room.mu.Unlock()
		if len(room.conns) == 0 {
			mu.Lock()
			delete(commMap, roomKey)
			mu.Unlock()
		}
	}()

	if userChan, ok := room.conns[connId]; ok {
		close(userChan)
		delete(room.conns, connId)
	}
}

// Returns all connections in a room
func GetRoomConnections(instanceId, roomId string) []string {
	roomKey := getRoomKey(instanceId, roomId)

	mu.Lock()
	room, ok := commMap[roomKey]
	mu.Unlock()

	if !ok {
		return []string{}
	}

	room.mu.Lock()
	defer room.mu.Unlock()

	keysIter := maps.Keys(room.conns)
	var keys []string
	for key := range keysIter {
		keys = append(keys, key)
	}
	return keys
}
