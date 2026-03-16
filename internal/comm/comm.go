package comm

import (
	"fmt"
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
	destRoom string
	destConn string
	srcRoom  string
	srcConnn string
	payload  string
}

type CommRoom struct {
	conns map[string]chan *CommEvent
	mu    sync.Mutex
}

var commMap map[string]*CommRoom
var mu sync.Mutex

func SendEvent(e *CommEvent) error {
	mu.Lock()
	room, ok := commMap[e.destRoom]
	mu.Unlock()

	if !ok {
		return fmt.Errorf("Invalid destination room")
	}

	room.mu.Lock()
	userChan, ok := room.conns[e.destConn]
	room.mu.Unlock()

	if !ok {
		return fmt.Errorf("Invalid destination user")
	}

	userChan <- e
	return nil
}
