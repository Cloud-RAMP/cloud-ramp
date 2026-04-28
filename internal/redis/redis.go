package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/Cloud-RAMP/cloud-ramp.git/internal/cfg"
	"github.com/Cloud-RAMP/cloud-ramp.git/internal/comm"
	"github.com/redis/go-redis/v9"
)

var client *redis.Client

// Returns the key for pub/sub for a given room
func getEventKey(instanceId, roomId string) string {
	return fmt.Sprintf("%s:events", comm.GetRoomKey(instanceId, roomId))
}

// Returns the key for users for a given room
func getUsersKey(instanceId, roomId string) string {
	return fmt.Sprintf("%s:users", comm.GetRoomKey(instanceId, roomId))
}

// returns the key for data for a given room
func getDataKey(instanceId, roomId string) string {
	return fmt.Sprintf("%s:data", comm.GetRoomKey(instanceId, roomId))
}

func getReconnectionKey(instanceId, roomId, ip string) string {
	return fmt.Sprintf("%s:%s", comm.GetRoomKey(instanceId, roomId), ip)
}

// Initialize the redis client. To be called on startup
func InitClient(ctx context.Context) error {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		return fmt.Errorf("could not get REDIS_URL from .env")
	}

	options, err := redis.ParseURL(redisURL)
	if err != nil {
		return err
	}

	client = redis.NewClient(options)
	roomChans = make(map[string]*pubSub)

	return nil
}

// Check if a user ID exists in redis. If it does, return it. Else empty string
func CheckUserID(ctx context.Context, instanceId, roomId, ip string) (string, error) {
	key := getReconnectionKey(instanceId, roomId, ip)
	uidResp := client.Get(ctx, key)
	if uidResp.Err() != nil {
		if errors.Is(uidResp.Err(), redis.Nil) {
			return "", nil
		}
		return "", uidResp.Err()
	}

	return uidResp.Result()
}

// Makes a user join a given room. If the room does not exist, it is created
//
// Broadcasts to all users in the room that they have joined
func JoinRoom(ctx context.Context, instanceId, roomId, userId string) (<-chan *redis.Message, error) {
	key := getUsersKey(instanceId, roomId)
	eventKey := getEventKey(instanceId, roomId)

	// "room" is a set of users
	err := client.SAdd(ctx, key, userId).Err()
	if err != nil {
		return nil, err
	}

	// add pub/sub object to the internal map
	roomChansMu.Lock()
	pubSub, ok := roomChans[key]
	if !ok {
		redisPubSub := client.Subscribe(ctx, eventKey)
		pubSub = initPubSub(redisPubSub)
		roomChans[key] = pubSub
	}
	roomChansMu.Unlock()

	// add the user to the pub/sub model and get the chan back
	ch := pubSub.addUser(userId)

	// publish initial JOIN event
	// TODO: add user config to toggle this on / off
	event := comm.CommEvent{
		DstConn:   "*",
		SrcConn:   userId,
		EventType: comm.JOIN,
	}
	eventJson, err := json.Marshal(event)
	if err != nil {
		return nil, err
	}
	if err = client.Publish(ctx, eventKey, eventJson).Err(); err != nil {
		return nil, err
	}

	// return chan for calling process to read from
	return ch, nil
}

// Removes a user from a room
//
// If the room is empty, its subscription is deleted
func LeaveRoom(ctx context.Context, instanceId, roomId, userId, ip string) error {
	key := getUsersKey(instanceId, roomId)

	// remove client from set
	err := client.SRem(ctx, key, userId).Err()
	if err != nil {
		return err
	}

	sCardRes := client.SCard(ctx, key)
	if sCardRes.Err() != nil {
		return sCardRes.Err()
	}
	roomSize := sCardRes.Val()

	reconnectKey := getReconnectionKey(instanceId, roomId, ip)
	if err := client.Set(ctx, reconnectKey, userId, cfg.IP_RECONNECT_TTL).Err(); err != nil {
		return err
	}

	// remove PubSubs for empty rooms
	if roomSize == 0 {
		// Delete empty data from redis (avoid using too much storage)
		if err := client.Del(ctx, key).Err(); err != nil {
			return err
		}
		if err := client.Del(ctx, getEventKey(instanceId, roomId)).Err(); err != nil {
			return err
		}

		roomChansMu.Lock()
		defer roomChansMu.Unlock()

		pubSub, ok := roomChans[key]
		if !ok {
			return nil
		}

		delete(roomChans, key)
		pubSub.removeUser(userId)
		pubSub.cancel() // cancel context to terminate goroutine
		return pubSub.dataChan.Close()
	}

	roomChansMu.Lock()
	pubSub, ok := roomChans[key]
	if ok {
		pubSub.removeUser(userId)
	}
	roomChansMu.Unlock()

	// Send leave event
	// Similarly, add config to turn this on/off
	event := comm.CommEvent{
		DstConn:   "*",
		SrcConn:   userId,
		Room:      roomId,
		EventType: comm.LEAVE,
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

// Returns all users for a given room
func GetAllUsers(ctx context.Context, instanceId, roomId string) ([]string, error) {
	key := getUsersKey(instanceId, roomId)
	sMembersRes := client.SMembers(ctx, key)
	if err := sMembersRes.Err(); err != nil {
		return nil, err
	}
	return sMembersRes.Val(), nil
}

// Get the k/v store from a certain key from an instance/room
func GetDataValue(ctx context.Context, instanceId, roomId string, key string) (string, error) {
	fullKey := fmt.Sprintf("%s:%s", getDataKey(instanceId, roomId), key)
	return client.Get(ctx, fullKey).Result()
}

// Set the k/v store at a certain key at a given instance/room
func SetDataValue(ctx context.Context, instanceId, roomId string, key string, value string) error {
	fullKey := fmt.Sprintf("%s:%s", getDataKey(instanceId, roomId), key)
	err := client.Set(ctx, fullKey, value, 0).Err()
	return err
}

func Delete(ctx context.Context, instanceId, roomId, key string) error {
	fullKey := fmt.Sprintf("%s:%s", getDataKey(instanceId, roomId), key)
	delRes := client.Del(ctx, fullKey)
	return delRes.Err()
}
