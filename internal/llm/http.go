package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// httpClient is shared across providers with a generous timeout for LLM calls.
var httpClient = &http.Client{Timeout: 120 * time.Second}

// postJSON sends body as JSON to url with the given headers and decodes the
// JSON response into out. Non-2xx responses return an error including the body.
func postJSON(ctx context.Context, url string, headers map[string]string, body any, out any) error {
	buf, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("llm request failed (%d): %s", resp.StatusCode, string(data))
	}
	return json.Unmarshal(data, out)
}
