package redis

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

func getRateKey(ip string) string {
	return fmt.Sprintf("rate:%s", ip)
}

func GetCurrentRequests(ip string) (int, error) {
	key := getRateKey(ip)

	getResp := client.Get(context.Background(), key)
	if getResp.Err() == redis.Nil {
		return 0, nil
	}
	if getResp.Err() != nil {
		return 0, getResp.Err()
	}

	numReqsString := getResp.Val()
	if numReqsString == "" {
		return 0, nil
	}

	numReqs, err := strconv.Atoi(numReqsString)
	if err != nil {
		return 0, err
	}

	return numReqs, nil
}

func SetCurrentRequests(ip string, numReqs int, ttl time.Duration) error {
	key := getRateKey(ip)

	return client.Set(context.Background(), key, numReqs, ttl).Err()
}
