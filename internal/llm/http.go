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

// researchClient allows the extra latency of live web-search research. A
// client's Timeout is a hard ceiling that context cannot lengthen (the minimum
// of the two applies), so research needs its own client rather than the shared
// 120s one.
var researchClient = &http.Client{Timeout: 5 * time.Minute}

// postJSON sends body as JSON to url with the given headers and decodes the
// JSON response into out, using the shared 120s client.
func postJSON(ctx context.Context, url string, headers map[string]string, body any, out any) error {
	return postJSONWith(httpClient, ctx, url, headers, body, out)
}

// postJSONWith is postJSON with an explicit client, used for research calls that
// need a longer timeout. Non-2xx responses return an error including the body.
func postJSONWith(client *http.Client, ctx context.Context, url string, headers map[string]string, body any, out any) error {
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
	resp, err := client.Do(req)
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
