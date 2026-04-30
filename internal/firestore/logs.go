package firestore

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Cloud-RAMP/cloud-ramp.git/internal/cfg"
	"github.com/Cloud-RAMP/cloud-ramp.git/internal/logger"
)

func init() {
	if !cfg.USE_FIRESTORE {
		return
	}

	ticker := time.NewTicker(cfg.LOG_DUMP_INTERVAL_SECONDS * time.Second)
	go func() {
		for range ticker.C {
			logger.ServerInfo("Dumping Firestore logs")
			err := OnLogDump()
			if err != nil {
				logger.ServerError("Dumping logs", err)
			}
		}
	}()
}

func OnLogDump() error {
	const root = "/tmp/cloudramp/logs"

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		// invalid file present?
		nameSplit := strings.Split(d.Name(), ".")
		if len(nameSplit) != 2 {
			return fmt.Errorf("Invalid filename: %s", d.Name())
		}
		if nameSplit[0] == "" || nameSplit[0] == "unknown" {
			return fmt.Errorf("Invalid filename: %s", d.Name())
		}

		if err = dumpSingleServiceLogs(path, nameSplit[0]); err != nil {
			return err
		}

		return nil
	})

	return err
}

func dumpSingleServiceLogs(path, instanceId string) error {
	contents, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	client, err := Client()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, _, err = client.
		Collection("services").
		Doc(instanceId).
		Collection("logs").
		Add(ctx, map[string]any{
			"content":   string(contents),
			"createdAt": time.Now().UTC(),
		})
	if err != nil {
		return fmt.Errorf("failed writing log doc for instance %s: %w", instanceId, err)
	}

	err = logger.RemoveLogger(instanceId)
	if err != nil {
		return err
	}
	return nil
}
