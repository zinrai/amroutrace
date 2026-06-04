package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/goccy/go-yaml"
)

// rewrittenConfigName derives the output file name from the input config path:
// the base name with an "amroutrace-" prefix. The same input always maps to
// the same output, so repeated runs overwrite a single file rather than
// accumulating. The file is written to the current directory.
func rewrittenConfigName(configPath string) string {
	return "amroutrace-" + filepath.Base(configPath)
}

// generateRewrittenConfig reads the production config at configPath, rewrites
// every receiver to deliver to captureBaseURL/<receiver-name>, writes the
// result to amroutrace-<base name> in the current directory, and returns that
// path. Both inject and rewrite use this so that what rewrite produces is
// exactly what inject feeds to Alertmanager.
func generateRewrittenConfig(configPath, captureBaseURL string) (string, error) {
	src, err := os.ReadFile(configPath)
	if err != nil {
		return "", fmt.Errorf("read config: %w", err)
	}

	rewritten, err := rewriteConfigToWebhook(src, captureBaseURL)
	if err != nil {
		return "", err
	}

	outPath := rewrittenConfigName(configPath)
	if err := os.WriteFile(outPath, rewritten, 0o600); err != nil {
		return "", fmt.Errorf("write rewritten config: %w", err)
	}

	return outPath, nil
}

// rewriteConfigToWebhook takes a rendered Alertmanager configuration and
// returns an equivalent configuration in which every receiver's notification
// settings are replaced by a single webhook_config pointing at
// captureBaseURL/<receiver-name>. The route tree and receiver names are
// preserved, so routing behavior is unchanged; only the delivery target is
// redirected to the local capture endpoint.
//
// This is the only place that depends on the Alertmanager configuration
// schema. It is kept separate from alert injection so that knowledge of the
// config format stays in one file.
func rewriteConfigToWebhook(src []byte, captureBaseURL string) ([]byte, error) {
	// Decode into a generic map so that fields amroutrace does not care about
	// (global, route, templates, inhibit_rules, time_intervals, ...) pass
	// through untouched.
	var cfg map[string]any
	if err := yaml.Unmarshal(src, &cfg); err != nil {
		return nil, fmt.Errorf("parse alertmanager config: %w", err)
	}

	raw, ok := cfg["receivers"]
	if !ok {
		return nil, fmt.Errorf("alertmanager config has no receivers")
	}
	list, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("receivers is not a list")
	}

	rewritten := make([]any, 0, len(list))
	for i, item := range list {
		recv, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("receiver #%d is not a mapping", i+1)
		}
		name, ok := recv["name"].(string)
		if !ok || name == "" {
			return nil, fmt.Errorf("receiver #%d has no name", i+1)
		}

		// Rebuild the receiver with only a webhook_config pointing at the
		// capture endpoint. Dropping every other *_configs (slack, email,
		// pagerduty, ...) and their parameters is intentional: routing is
		// determined by labels and the route tree, not by the delivery
		// mechanism or notification parameters such as send_resolved.
		// Rebuilding rather than editing keeps this independent of which
		// notifier types the source config happened to use.
		rewritten = append(rewritten, map[string]any{
			"name": name,
			"webhook_configs": []any{
				map[string]any{
					"url": fmt.Sprintf("%s/%s", captureBaseURL, name),
				},
			},
		})
	}

	cfg["receivers"] = rewritten

	// Drop inhibit_rules. Inhibition suppresses one alert's notification when
	// another is firing, which is a concern separate from routing. Left in
	// place, it could stop an alert from reaching a receiver it routes to and
	// make verification report a false "not delivered". amroutrace verifies
	// which receivers an input passes through, not inhibition, so it is removed
	// to keep the dynamic result aligned with amroutify's static routing.
	delete(cfg, "inhibit_rules")

	out, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("marshal rewritten config: %w", err)
	}
	return out, nil
}
