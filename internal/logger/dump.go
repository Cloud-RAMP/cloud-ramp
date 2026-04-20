package logger

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Cloud-RAMP/cloud-ramp.git/internal/cfg"
	"github.com/Cloud-RAMP/cloud-ramp.git/internal/firestore"
)

func init() {
	if !cfg.USE_FIRESTORE {
		return
	}

	ticker := time.NewTicker(cfg.LOG_DUMP_INTERVAL_SECONDS * time.Second)
	go func() {
		for range ticker.C {
			OnDump()
		}
	}()
}

func OnDump() {
	const root = "/tmp/cloudramp/logs"

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}

		// invalid file present?
		nameSplit := strings.Split(d.Name(), ".")
		if len(nameSplit) != 2 {
			return nil
		}
		if nameSplit[0] == "" || nameSplit[0] == "unknown" {
			return nil
		}

		if err = dumpSingleServiceLogs(path, nameSplit[0]); err != nil {
			fmt.Println("Failed to dump service logs:", nameSplit[0], err)
			return nil
		}

		return nil
	})
	if err != nil {
		fmt.Println("Logger failed to walk log directory")
	}
}

func dumpSingleServiceLogs(path, instanceId string) error {
	contents, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	client, err := firestore.Client()
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

	err = removeLogger(instanceId)
	if err != nil {
		return err
	}
	return nil
}
