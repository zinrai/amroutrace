package main

import (
	"flag"
	"fmt"
	"os"
)

func runVerify(args []string) error {
	fs := flag.NewFlagSet("verify", flag.ExitOnError)
	testsPath := fs.String("tests", "", "Path to routing_tests.yml (required)")
	arrivalsPath := fs.String("arrivals", "",
		"Path to the arrivals file: JSON lines, one delivery per line, each mapping a receiver to a test case (required)")
	fs.Parse(args)

	if *testsPath == "" {
		return fmt.Errorf("-tests is required")
	}
	if *arrivalsPath == "" {
		return fmt.Errorf("-arrivals is required")
	}

	spec, err := loadSpec(*testsPath)
	if err != nil {
		return err
	}

	f, err := os.Open(*arrivalsPath)
	if err != nil {
		return fmt.Errorf("open arrivals: %w", err)
	}
	defer f.Close()

	arrivals, err := readArrivals(f)
	if err != nil {
		return fmt.Errorf("read arrivals: %w", err)
	}

	rep := compare(spec, arrivals)

	if err := writeJSONReport(os.Stdout, rep); err != nil {
		return err
	}

	// Non-zero exit on failure so CI can gate on the exit code without
	// parsing the JSON.
	if !rep.allPassed() {
		os.Exit(1)
	}
	return nil
}
