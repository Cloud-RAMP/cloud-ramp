package main

import (
	"context"

	"github.com/Cloud-RAMP/cloud-ramp.git/internal/server"
)

func main() {
	server.Start(context.Background())
}
