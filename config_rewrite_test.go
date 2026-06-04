package main

import (
	"fmt"
	"sort"
	"testing"

	"github.com/goccy/go-yaml"
)

// rewriteConfigToWebhook is the most intricate transform in the tool, and a bug
// here silently changes which receivers an alert appears to reach, breaking the
// whole premise. These tests pin the invariants the transform must satisfy,
// stated as rules over the output structure rather than as a list of expected
// values, so they do not depend on the particular input config.
func TestRewriteConfigToWebhook(t *testing.T) {
	const base = "http://127.0.0.1:8000"

	// A synthetic config exercising the receiver shapes the transform must
	// handle: no notification block, a non-webhook notifier, a webhook
	// notifier, and notifiers carrying parameters that must not survive.
	src := []byte(`
route:
  receiver: r-plain
  routes:
    - match:
        k: v
      receiver: r-none
receivers:
  - name: r-none
  - name: r-plain
    slack_configs:
      - api_url: https://example.invalid/x
        channel: "#c"
        send_resolved: false
  - name: r-webhook
    webhook_configs:
      - url: https://example.invalid/y
        send_resolved: true
inhibit_rules:
  - source_match: {severity: critical}
    target_match: {severity: warning}
    equal: [host]
templates:
  - /etc/alertmanager/templates/x.tmpl
`)
	inputReceivers := []string{"r-none", "r-plain", "r-webhook"}

	out, err := rewriteConfigToWebhook(src, base)
	if err != nil {
		t.Fatalf("rewriteConfigToWebhook: %v", err)
	}

	var cfg map[string]any
	if err := yaml.Unmarshal(out, &cfg); err != nil {
		t.Fatalf("rewritten config does not parse: %v", err)
	}

	// inhibit_rules must be removed: inhibition could suppress delivery and
	// make verification report a false "not delivered".
	if _, ok := cfg["inhibit_rules"]; ok {
		t.Error("inhibit_rules should be present nowhere in the output")
	}

	// The route tree must be preserved unchanged: it determines which receiver
	// an alert reaches, which is exactly what is being verified.
	if _, ok := cfg["route"]; !ok {
		t.Error("route should be preserved")
	}

	receivers := receiverList(t, cfg)

	// The set of receiver names must be unchanged: every receiver is rewritten,
	// none added or dropped, regardless of whether it had a notification block.
	assertSameSet(t, "receiver names", receiverNames(receivers), inputReceivers)

	for _, recv := range receivers {
		name, _ := recv["name"].(string)

		// A receiver must consist of exactly name and webhook_configs: nothing
		// from the source receiver (slack_configs, channel, ...) may remain.
		assertExactKeys(t, fmt.Sprintf("receiver %q", name), recv, "name", "webhook_configs")

		blocks, ok := recv["webhook_configs"].([]any)
		if !ok || len(blocks) != 1 {
			t.Errorf("receiver %q: want exactly one webhook_config", name)
			continue
		}
		block, ok := blocks[0].(map[string]any)
		if !ok {
			t.Errorf("receiver %q: webhook_config is not a mapping", name)
			continue
		}

		// A webhook_config must consist of exactly url: no other parameter
		// (send_resolved, http_config, ...) may be carried over, since routing
		// does not depend on delivery parameters.
		assertExactKeys(t, fmt.Sprintf("receiver %q webhook_config", name), block, "url")

		// The url must follow the <base>/<name> rule, so the request path
		// identifies the receiver.
		want := base + "/" + name
		if block["url"] != want {
			t.Errorf("receiver %q: url = %v, want %q", name, block["url"], want)
		}
	}
}

func TestRewriteConfigToWebhookErrors(t *testing.T) {
	cases := map[string]string{
		"not yaml":     `: : :`,
		"no receivers": "route:\n  receiver: x\n",
	}
	for name, src := range cases {
		if _, err := rewriteConfigToWebhook([]byte(src), "http://127.0.0.1:8000"); err == nil {
			t.Errorf("%s: expected error, got nil", name)
		}
	}
}

func receiverList(t *testing.T, cfg map[string]any) []map[string]any {
	t.Helper()
	raw, ok := cfg["receivers"].([]any)
	if !ok {
		t.Fatal("receivers is not a list")
	}
	out := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		recv, ok := item.(map[string]any)
		if !ok {
			t.Fatal("receiver is not a mapping")
		}
		out = append(out, recv)
	}
	return out
}

func receiverNames(receivers []map[string]any) []string {
	names := make([]string, 0, len(receivers))
	for _, r := range receivers {
		name, _ := r["name"].(string)
		names = append(names, name)
	}
	return names
}

// assertExactKeys fails unless m has exactly the given keys, no more, no less.
func assertExactKeys(t *testing.T, what string, m map[string]any, keys ...string) {
	t.Helper()
	want := make(map[string]bool, len(keys))
	for _, k := range keys {
		want[k] = true
		if _, ok := m[k]; !ok {
			t.Errorf("%s: missing key %q", what, k)
		}
	}
	for k := range m {
		if !want[k] {
			t.Errorf("%s: unexpected key %q", what, k)
		}
	}
}

func assertSameSet(t *testing.T, what string, got, want []string) {
	t.Helper()
	g := append([]string(nil), got...)
	w := append([]string(nil), want...)
	sort.Strings(g)
	sort.Strings(w)
	if fmt.Sprint(g) != fmt.Sprint(w) {
		t.Errorf("%s: got %v, want %v", what, g, w)
	}
}
