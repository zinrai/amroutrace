package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// caseLabel is the label injected into every alert to identify which test case
// it originated from. The verify step groups captured requests by this label.
// It is the only contract shared between injection and verification.
const caseLabel = "e2e_case"

// alert is the subset of the Alertmanager v2 alert schema this tool sends.
// Only labels are sent: amroutrace verifies which receivers an alert routes
// to, which is determined by labels alone. startsAt and endsAt are omitted, so
// Alertmanager fills them in (treating the alert as firing); the tool does not
// model resolution.
type alert struct {
	Labels map[string]string `json:"labels"`
}

// alertmanagerClient posts alerts to a single Alertmanager instance.
type alertmanagerClient struct {
	baseURL string
	http    *http.Client
}

// newAlertmanagerClient returns a client targeting the Alertmanager at baseURL
// (for example "http://127.0.0.1:9093").
func newAlertmanagerClient(baseURL string, timeout time.Duration) *alertmanagerClient {
	return &alertmanagerClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: timeout},
	}
}

// inject posts a single alert carrying the given labels plus the e2e_case
// label set to caseName. Only labels are sent; Alertmanager treats the alert
// as firing and routes it according to the configured routing tree.
func (c *alertmanagerClient) inject(ctx context.Context, caseName string, labels map[string]string) error {
	merged := make(map[string]string, len(labels)+1)
	for k, v := range labels {
		merged[k] = v
	}
	// The case label is set last so a stray e2e_case in the spec cannot
	// silently override the injected value.
	merged[caseLabel] = caseName

	payload := []alert{{Labels: merged}}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal alert for case %q: %w", caseName, err)
	}

	url := c.baseURL + "/api/v2/alerts"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request for case %q: %w", caseName, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("post alert for case %q: %w", caseName, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("post alert for case %q: status %d: %s",
			caseName, resp.StatusCode, strings.TrimSpace(string(snippet)))
	}

	return nil
}
