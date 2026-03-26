package comm

import (
	"encoding/json"
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

var commEventTypeToString = map[CommEventType]string{
	SEND_MESSAGE: "SEND_MESSAGE",
	BROADCAST:    "BROADCAST",
	LEAVE:        "LEAVE",
	JOIN:         "JOIN",
}

func (c CommEventType) MarshalJSON() ([]byte, error) {
	str, ok := commEventTypeToString[c]
	if !ok {
		return nil, fmt.Errorf("invalid CommEventType: %d", c)
	}
	return json.Marshal(str)
}

type CommEvent struct {
	Instance  string        `json:"-"` // don't add instance to JSON
	Room      string        `json:"-"` // don't add room to JSON (receving user should know what room they are in)
	DstConn   string        `json:"dst_conn"`
	SrcConn   string        `json:"src_conn"`
	Payload   string        `json:"payload"`
	EventType CommEventType `json:"event_type"`
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

// GetRoomKey creates a compound key from instance and room
func GetRoomKey(instanceId, roomId string) string {
	return fmt.Sprintf("%s:%s", instanceId, roomId)
}

// Send a CommEvent to a dst user
func SendEvent(e *CommEvent) error {
	roomKey := GetRoomKey(e.Instance, e.Room)

	mu.Lock()
	room, ok := commMap[roomKey]
	mu.Unlock()

	if !ok {
		return fmt.Errorf("invalid destination instance/room: %s/%s", e.Instance, e.Room)
	}

	room.mu.Lock()
	userChan, ok := room.conns[e.DstConn]
	room.mu.Unlock()

	if !ok {
		return fmt.Errorf("invalid destination connection: %s", e.DstConn)
	}

	// non-blocking, avoid deadlock
	select {
	case userChan <- e:
		return nil
	default:
		return fmt.Errorf("channel full or unavailable for connection: %s", e.DstConn)
	}
}

// Initialize a message channel for a given connection
func InitConn(instanceId, roomId, connId string) <-chan *CommEvent {
	roomKey := GetRoomKey(instanceId, roomId)

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

	return room.conns[connId]
}

// Close the message channel for a given connection
func CloseConn(instanceId, roomId, connId string) {
	roomKey := GetRoomKey(instanceId, roomId)

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
	roomKey := GetRoomKey(instanceId, roomId)

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

// Returns the event channel for a given connection
func GetEventChan(instanceId, roomId, connId string) <-chan *CommEvent {
	roomKey := GetRoomKey(instanceId, roomId)

	mu.Lock()
	room, ok := commMap[roomKey]
	mu.Unlock()

	if !ok {
		return nil
	}

	room.mu.Lock()
	defer room.mu.Unlock()

	userChan, ok := room.conns[connId]
	if !ok {
		return nil
	}

	return userChan
}
