package sandbox

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/Cloud-RAMP/cloud-ramp.git/internal/logger"
)

func LoaderFunction(ctx context.Context, moduleId string) ([]byte, error) {
	logger.Info(moduleId, "Loading module from external store")

	baseURL := os.Getenv("BLOB_BASE_URL")
	if baseURL == "" {
		logger.Error(moduleId, "Failed to get base URL from env")
		return nil, fmt.Errorf("Failed to base URL from env")
	}

	blobURL := fmt.Sprintf("%s/%s", baseURL, moduleId)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, blobURL, nil)
	if err != nil {
		logger.Error(moduleId, fmt.Sprintf("Failed to create new blob store request: %e", err))
		return nil, err
	}

	// Privately read data
	if tok := os.Getenv("BLOB_READ_WRITE_TOKEN"); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logger.Error(moduleId, fmt.Sprintf("Failed to execute blob store request: %e", err))
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		err = fmt.Errorf("blob request failed: %s: %s", resp.Status, string(body))
		logger.Error(moduleId, fmt.Sprintf("Request failed with abnormal status code: %e", err))
		return nil, err
	}

	return io.ReadAll(resp.Body)
}

func DummyLoaderFunction(ctx context.Context, moduleId string) ([]byte, error) {
	file, err := os.ReadFile("../wasm-sandbox/example/build/release.wasm")
	if err != nil {
		return nil, err
	}

	return file, nil
}
