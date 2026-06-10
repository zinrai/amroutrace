package main

import (
	"fmt"
	"os"

	"github.com/goccy/go-yaml"
)

// Spec is the top-level structure of routing_tests.yml. The format is
// identical to the one consumed by amroutify, so a single file acts as the
// single source of truth for both the static and end-to-end checks.
type Spec struct {
	Tests []Test `yaml:"tests"`
}

// Test is a single routing test case.
//
// Labels are the alert labels to inject. ExpectedReceivers is the set of
// receivers the alert is expected to reach; order does not matter and the
// empty set means the alert is expected to reach no receiver.
type Test struct {
	Name              string            `yaml:"name"`
	Labels            map[string]string `yaml:"labels"`
	ExpectedReceivers []string          `yaml:"expected_receivers"`
}

// loadSpec reads and parses the test specification at path.
func loadSpec(path string) (*Spec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read test spec: %w", err)
	}

	var spec Spec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("parse test spec: %w", err)
	}

	if err := spec.validate(); err != nil {
		return nil, err
	}

	return &spec, nil
}

func (s *Spec) validate() error {
	if len(s.Tests) == 0 {
		return fmt.Errorf("test spec contains no tests")
	}

	seen := make(map[string]bool, len(s.Tests))
	for i, t := range s.Tests {
		if t.Name == "" {
			return fmt.Errorf("test #%d has no name", i+1)
		}
		if seen[t.Name] {
			return fmt.Errorf("duplicate test name %q", t.Name)
		}
		seen[t.Name] = true

		if len(t.Labels) == 0 {
			return fmt.Errorf("test %q has no labels", t.Name)
		}
	}

	return nil
}
