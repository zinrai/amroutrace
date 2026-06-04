package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// arrival is one observed delivery: an alert carrying caseName reached the
// receiver named receiver.
type arrival struct {
	caseName string
	receiver string
}

// arrivalLine is the JSON-lines wire format amroutrace reads: one delivery per
// line. This intermediate format decouples amroutrace from whatever tool
// captured the requests. Producing it from a specific capture tool's output is
// out of scope for amroutrace (see the README for an http-request-capture
// example).
type arrivalLine struct {
	Receiver string `json:"receiver"`
	Case     string `json:"case"`
}

// readArrivals reads JSON-lines arrival records from r. Each line must be an
// object with "receiver" and "case" string fields. Blank lines are skipped; a
// line that is not valid JSON, or that omits either field, is an error.
func readArrivals(r io.Reader) ([]arrival, error) {
	var arrivals []arrival

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var rec arrivalLine
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNo, err)
		}
		if rec.Receiver == "" {
			return nil, fmt.Errorf("line %d: missing receiver", lineNo)
		}
		if rec.Case == "" {
			return nil, fmt.Errorf("line %d: missing case", lineNo)
		}

		arrivals = append(arrivals, arrival{caseName: rec.Case, receiver: rec.Receiver})
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return arrivals, nil
}
