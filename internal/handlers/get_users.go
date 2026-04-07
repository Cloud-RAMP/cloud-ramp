package handlers

import (
	"context"
	"strings"

	"github.com/Cloud-RAMP/cloud-ramp.git/internal/redis"
	wasmevents "github.com/Cloud-RAMP/wasm-sandbox/pkg/wasm-events"
)

func GetUsersHandler(event *wasmevents.WASMEventInfo) (string, error) {
	users, err := redis.GetAllUsers(context.Background(), event.InstanceId, event.RoomId)
	if err != nil {
		return "", err
	}
  
	return strings.Join(users, ","), nil
}	
