package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"time"
)

// alertmanagerProcess manages the lifecycle of a local Alertmanager started by
// amroutrace for the duration of a test run. amroutrace starts its own
// Alertmanager with a webhook-rewritten config so that it never touches a
// production instance: there is no path by which injected alerts can reach a
// production Alertmanager.
type alertmanagerProcess struct {
	cmd     *exec.Cmd
	baseURL string
}

// startAlertmanager launches the Alertmanager binary at binPath with the given
// config file, listening on listenAddr (host:port). It returns once the
// process has been started; call waitReady before injecting.
func startAlertmanager(binPath, configPath, listenAddr string) (*alertmanagerProcess, error) {
	cmd := exec.Command(binPath,
		"--config.file="+configPath,
		"--web.listen-address="+listenAddr,
	)
	// Surface Alertmanager's own logs so failures are visible to the operator.
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start alertmanager: %w", err)
	}

	return &alertmanagerProcess{
		cmd:     cmd,
		baseURL: "http://" + listenAddr,
	}, nil
}

// waitReady polls Alertmanager's readiness endpoint until it responds 200 or
// the timeout elapses.
func (p *alertmanagerProcess) waitReady(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: time.Second}

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if p.ready(ctx, client) {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}

	return fmt.Errorf("alertmanager did not become ready within %s", timeout)
}

// ready reports whether Alertmanager's readiness endpoint returns 200.
func (p *alertmanagerProcess) ready(ctx context.Context, client *http.Client) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/-/ready", nil)
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// stop terminates the Alertmanager process and waits for it to exit. It is safe
// to call more than once.
func (p *alertmanagerProcess) stop() {
	if p.cmd == nil || p.cmd.Process == nil {
		return
	}
	// SIGTERM lets Alertmanager shut down cleanly; Wait reaps the process.
	_ = p.cmd.Process.Signal(os.Interrupt)
	done := make(chan struct{})
	go func() {
		_ = p.cmd.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		_ = p.cmd.Process.Kill()
		<-done
	}
	p.cmd = nil
}
