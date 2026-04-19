package redis

import (
	"context"
	"fmt"
	"strconv"

	"github.com/Cloud-RAMP/cloud-ramp.git/internal/cfg"
)

func getRateKey(ip string) string {
	return fmt.Sprintf("rate:%s", ip)
}

func GetCurrentRequests(ip string) (int, error) {
	key := getRateKey(ip)

	getResp := client.Get(context.Background(), key)
	if getResp.Err() != nil {
		return 0, getResp.Err()
	}

	numReqsString := getResp.Val()
	numReqs, err := strconv.Atoi(numReqsString)
	if err != nil {
		return 0, err
	}

	return numReqs, nil
}

func SetCurrentRequests(ip string, numReqs int) error {
	key := getRateKey(ip)

	return client.Set(context.Background(), key, numReqs, cfg.RATE_TTL).Err()
}
