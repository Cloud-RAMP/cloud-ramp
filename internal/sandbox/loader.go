package sandbox

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
)

func LoaderFunction(ctx context.Context, moduleId string) ([]byte, error) {
	baseURL := os.Getenv("BLOB_BASE_URL")
	if baseURL == "" {
		fmt.Println("Failed to get base url from env")
		return nil, fmt.Errorf("Failed to base URL from env")
	}

	blobURL := fmt.Sprintf("%s/%s", baseURL, moduleId)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, blobURL, nil)
	if err != nil {
		return nil, err
	}

	// Privately read data
	if tok := os.Getenv("BLOB_READ_WRITE_TOKEN"); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("blob request failed: %s: %s", resp.Status, string(body))
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
