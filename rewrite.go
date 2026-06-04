package main

import (
	"flag"
	"fmt"
)

// runRewrite generates the webhook-rewritten config and stops. It is the
// front half of inject: a dry run that lets the operator inspect exactly what
// inject would feed to Alertmanager.
func runRewrite(args []string) error {
	fs := flag.NewFlagSet("rewrite", flag.ExitOnError)
	configPath := fs.String("config", "", "Path to the rendered production alertmanager.yml to rewrite (required)")
	captureURL := fs.String("capture-url", "http://127.0.0.1:8000",
		"Base URL of the capture endpoint; receivers are rewritten to <capture-url>/<receiver-name>")
	fs.Parse(args)

	if *configPath == "" {
		return fmt.Errorf("-config is required")
	}

	outPath, err := generateRewrittenConfig(*configPath, *captureURL)
	if err != nil {
		return err
	}

	fmt.Printf("wrote %s\n", outPath)
	return nil
}
