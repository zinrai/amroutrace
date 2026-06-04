package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func runInject(args []string) error {
	fs := flag.NewFlagSet("inject", flag.ExitOnError)
	testsPath := fs.String("tests", "", "Path to routing_tests.yml (required)")
	configPath := fs.String("config", "", "Path to the rendered production alertmanager.yml to test (required)")
	amBin := fs.String("alertmanager-bin", "alertmanager", "Path to the alertmanager binary")
	listen := fs.String("listen", "127.0.0.1:9093", "Address Alertmanager listens on")
	captureURL := fs.String("capture-url", "http://127.0.0.1:8000",
		"Base URL of the capture endpoint; receivers are rewritten to <capture-url>/<receiver-name>")
	readyTimeout := fs.Duration("ready-timeout", 10*time.Second, "How long to wait for Alertmanager to become ready")
	timeout := fs.Duration("timeout", time.Second, "Per-request HTTP timeout")
	runFor := fs.Duration("run-for", 0,
		"Stop Alertmanager after this duration; 0 means run until interrupted (Ctrl-C)")
	fs.Parse(args)

	if *testsPath == "" {
		return fmt.Errorf("-tests is required")
	}
	if *configPath == "" {
		return fmt.Errorf("-config is required")
	}

	spec, err := loadSpec(*testsPath)
	if err != nil {
		return err
	}

	// Rewrite the production config so every receiver delivers to the local
	// capture endpoint, and write it to amroutrace-<base name> in the current
	// directory. amroutrace runs its own Alertmanager with this rewritten
	// config; it never touches a production instance. The file is left in
	// place after the run as a record of what was fed to Alertmanager.
	rewrittenPath, err := generateRewrittenConfig(*configPath, *captureURL)
	if err != nil {
		return err
	}
	fmt.Printf("wrote %s\n", rewrittenPath)

	// Start Alertmanager and make sure it is always stopped on the way out.
	am, err := startAlertmanager(*amBin, rewrittenPath, *listen)
	if err != nil {
		return err
	}
	defer am.stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := am.waitReady(ctx, *readyTimeout); err != nil {
		return err
	}

	// Inject every test case.
	client := newAlertmanagerClient(am.baseURL, *timeout)
	for _, t := range spec.Tests {
		if err := client.inject(ctx, t.Name, t.Labels); err != nil {
			return err
		}
		fmt.Printf("injected: %s\n", t.Name)
	}
	fmt.Printf("\ninjected %d case(s); Alertmanager running at %s\n", len(spec.Tests), am.baseURL)

	// Keep Alertmanager alive so that delivery, and later resolved
	// notifications, can reach the capture endpoint. Stop on Ctrl-C, or after
	// -run-for if set.
	waitForStop(ctx, *runFor)
	fmt.Println("stopping Alertmanager")
	return nil
}

// waitForStop blocks until an interrupt signal arrives, or until runFor elapses
// when runFor is non-zero.
func waitForStop(ctx context.Context, runFor time.Duration) {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sig)

	if runFor > 0 {
		fmt.Printf("running for %s (or until Ctrl-C)\n", runFor)
		select {
		case <-sig:
		case <-time.After(runFor):
		case <-ctx.Done():
		}
		return
	}

	fmt.Println("running until interrupted (Ctrl-C)")
	select {
	case <-sig:
	case <-ctx.Done():
	}
}
