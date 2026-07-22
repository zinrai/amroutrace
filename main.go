// Command amroutrace traces an alert through Alertmanager routing end to end.
//
// It has two operations, kept separate so the Alertmanager timing (group_wait,
// group_interval) stays under the operator's control rather than hidden inside
// a fixed wait:
//
//	inject  Post each test case's alert to the Alertmanager v2 API.
//	verify  Compare http-request-capture's log against expected_receivers.
//
// Between the two, http-request-capture records which receiver paths were
// reached. The single source of truth is routing_tests.yml, shared with
// amroutify, so the static and end-to-end checks assert against the same data.
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "inject":
		err = runInject(os.Args[2:])
	case "rewrite":
		err = runRewrite(os.Args[2:])
	case "verify":
		err = runVerify(os.Args[2:])
	case "version":
		printVersion()
		return
	case "-h", "--help", "help":
		usage()
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `amroutrace: end-to-end trace for Alertmanager routing

Usage:
  amroutrace rewrite -config alertmanager.yml
  amroutrace inject  -tests routing_tests.yml -config alertmanager.yml
  amroutrace verify  -tests routing_tests.yml -arrivals arrivals.jsonl

Commands:
  rewrite  Rewrite a production config so every receiver delivers to the local
           capture endpoint, and write it to amroutrace-<name>. A dry run of
           inject's front half.
  inject   Rewrite the config, start a local Alertmanager with it, and post
           each test case's alert.
  verify   Compare the arrivals against expected_receivers.
`)
}
