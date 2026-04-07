package firestore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"

	"cloud.google.com/go/firestore"
	gfirestore "cloud.google.com/go/firestore"
	"google.golang.org/api/option"
)

var (
	client   *gfirestore.Client
	initOnce sync.Once
	initErr  error
)

type serviceAccount struct {
	ProjectID string `json:"project_id"`
}

// InitClient initializes a singleton Firestore client.
// Safe to call multiple times.
func InitClient(ctx context.Context) (*gfirestore.Client, error) {
	initOnce.Do(func() {
		credsJSON := os.Getenv("FIREBASE_SERVICE_ACCOUNT_JSON")
		if credsJSON == "" {
			initErr = errors.New("FIREBASE_SERVICE_ACCOUNT_JSON is missing")
			return
		}

		projectID := os.Getenv("FIREBASE_PROJECT_ID")
		if projectID == "" {
			var sa serviceAccount
			if err := json.Unmarshal([]byte(credsJSON), &sa); err != nil {
				initErr = fmt.Errorf("failed to parse FIREBASE_SERVICE_ACCOUNT_JSON: %w", err)
				return
			}
			if sa.ProjectID == "" {
				initErr = errors.New("project_id missing in FIREBASE_SERVICE_ACCOUNT_JSON")
				return
			}
			projectID = sa.ProjectID
		}

		client, initErr = gfirestore.NewClient(
			ctx,
			projectID,
			option.WithCredentialsJSON([]byte(credsJSON)),
		)
	})

	return client, initErr
}

func Client() (*gfirestore.Client, error) {
	if client == nil {
		return nil, errors.New("firestore client is not initialized; call InitClient first")
	}
	return client, nil
}

func Close() error {
	if client == nil {
		return nil
	}
	return client.Close()
}

func Delete(ctx context.Context, instanceId, roomId, key string) error {
	c, err := Client()
	if err != nil {
		return err
	}

	_, err = c.Collection("services").
		Doc(instanceId).
		Collection("rooms").
		Doc(roomId).
		Update(ctx, []firestore.Update{
			{Path: key, Value: firestore.Delete},
		})
	if err != nil {
		return err
	}

	return nil
}

func Get(ctx context.Context, instanceId, roomId, key string) (string, error) {
	c, err := Client()
	if err != nil {
		return "", err
	}

	snap, err := c.Collection("services").
		Doc(instanceId).
		Collection("rooms").
		Doc(roomId).
		Get(ctx)
	if err != nil {
		return "", err
	}

	data := snap.Data()
	val, ok := data[key]
	if !ok {
		return "", fmt.Errorf("Key not found")
	}

	return fmt.Sprintf("%v", val), nil
}

func Set(ctx context.Context, instanceId, roomId, key, value string) error {
	c, err := Client()
	if err != nil {
		return err
	}

	_, err = c.Collection("services").
		Doc(instanceId).
		Collection("rooms").
		Doc(roomId).
		Set(ctx, map[string]any{
			key: value,
		}, firestore.MergeAll)
	if err != nil {
		return err
	}

	return nil
}
