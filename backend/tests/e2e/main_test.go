//go:build e2e

package e2e

import (
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"
)

var apiBase string

func TestMain(m *testing.M) {
	apiBase = os.Getenv("E2E_API_BASE")
	if apiBase == "" {
		apiBase = "http://localhost:8080"
	}

	if err := waitForAPI(30 * time.Second); err != nil {
		fmt.Fprintf(os.Stderr, "API not ready: %v\n", err)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

func waitForAPI(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 2 * time.Second}

	for time.Now().Before(deadline) {
		resp, err := client.Get(apiBase + "/api/v1/games/live")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode < 500 {
				return nil
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("API at %s did not become ready within %v", apiBase, timeout)
}
