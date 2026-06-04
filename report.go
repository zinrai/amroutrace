package main

import (
	"encoding/json"
	"io"
)

// writeJSONReport writes the report as indented JSON. The report is intended
// to be consumed by a harness (or read through jq); there is no separate
// human-formatted output.
func writeJSONReport(w io.Writer, rep *report) error {
	type jsonResult struct {
		Name       string   `json:"name"`
		Passed     bool     `json:"passed"`
		Expected   []string `json:"expected"`
		Actual     []string `json:"actual"`
		Missing    []string `json:"missing,omitempty"`
		Unexpected []string `json:"unexpected,omitempty"`
	}
	type jsonReport struct {
		Passed  bool         `json:"passed"`
		Total   int          `json:"total"`
		Results []jsonResult `json:"results"`
	}

	out := jsonReport{
		Passed:  rep.allPassed(),
		Total:   len(rep.Results),
		Results: make([]jsonResult, 0, len(rep.Results)),
	}
	for _, r := range rep.Results {
		out.Results = append(out.Results, jsonResult{
			Name:       r.Name,
			Passed:     r.Passed,
			Expected:   r.Expected,
			Actual:     r.Actual,
			Missing:    r.Missing,
			Unexpected: r.Unexpected,
		})
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
