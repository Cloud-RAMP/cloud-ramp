package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/Cloud-RAMP/cloud-ramp.git/internal/comm"
	"github.com/redis/go-redis/v9"
)

var roomChans map[string]*pubSub
var roomChansMu sync.Mutex

const userChanBufferSize = 32

type pubSub struct {
	ctx       context.Context
	cancel    context.CancelFunc
	dataChan  *redis.PubSub
	userChans map[string]chan *redis.Message
	mu        sync.RWMutex
}

// Intialize a pub/sub object for a room chan
//
// Starts a new goroutine to receive redis messages
func initPubSub(dataChan *redis.PubSub) *pubSub {
	ctx, cancel := context.WithCancel(context.Background())
	ps := &pubSub{
		ctx:       ctx,
		cancel:    cancel,
		dataChan:  dataChan,
		userChans: make(map[string]chan *redis.Message),
	}

	redisChan := dataChan.Channel()
	// detach goroutines to distribtue events from the single redis chan to all
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-redisChan:
				if !ok {
					return
				}

				// iterate over all message chans and fan out the redis message
				ps.mu.RLock()
				for _, userChan := range ps.userChans {
					select {
					case userChan <- event: // event delivered
					default: // not delivered, buffer full. delivery skipped
					}
				}
				ps.mu.RUnlock()
			}
		}
	}()

	return ps
}

// Add a user to the pubsub model for a room and return the message chan
func (ps *pubSub) addUser(userId string) <-chan *redis.Message {
	ch := make(chan *redis.Message, userChanBufferSize)

	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.userChans[userId] = ch
	return ch
}

// Remove a user from the pubsub model and close the chan
func (ps *pubSub) removeUser(userId string) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	ch, ok := ps.userChans[userId]
	if !ok {
		return
	}

	close(ch)
	delete(ps.userChans, userId)
}

// Broadcast a message to an entire room
func Broadcast(ctx context.Context, instanceId, roomId, userId, message string) error {
	event := comm.CommEvent{
		DstConn:   "*",
		SrcConn:   userId,
		Payload:   message,
		EventType: comm.BROADCAST,
	}
	eventJson, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if err = client.Publish(ctx, getEventKey(instanceId, roomId), eventJson).Err(); err != nil {
		return err
	}

	return nil
}

// Send a message that goes through redis
//
// This is only to be used if the destination user is not connected to the same node
func SendMessage(ctx context.Context, instanceId, roomId, userId, dstUserId, message string) error {
	event := comm.CommEvent{
		Room:      roomId,
		DstConn:   dstUserId,
		SrcConn:   userId,
		Payload:   message,
		EventType: comm.SEND_MESSAGE,
	}
	eventJson, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if err = client.Publish(ctx, getEventKey(instanceId, roomId), eventJson).Err(); err != nil {
		return err
	}

	return nil
}

// Send any commEvent. To be used in handler functions.
//
// To use this function, make sure the event you pass in has the Instance and Room fields set
func SendCommEvent(ctx context.Context, event *comm.CommEvent) error {
	if event == nil {
		return fmt.Errorf("event is nil")
	}

	eventJson, err := json.Marshal(event)
	if err != nil {
		return err
	}

	if err = client.Publish(ctx, getEventKey(event.Instance, event.Room), eventJson).Err(); err != nil {
		return err
	}
	return nil
}
