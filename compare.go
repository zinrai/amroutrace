package main

import "sort"

// caseResult is the outcome of verifying one test case.
type caseResult struct {
	Name       string
	Expected   []string // sorted
	Actual     []string // sorted
	Missing    []string // expected but not observed
	Unexpected []string // observed but not expected
	Passed     bool
}

// report is the outcome of verifying all test cases.
type report struct {
	Results []caseResult
}

// allPassed reports whether every test case passed.
func (r *report) allPassed() bool {
	for _, res := range r.Results {
		if !res.Passed {
			return false
		}
	}
	return true
}

// compare verifies each test in spec against the observed arrivals. Comparison
// is set-based and order-independent, matching amroutify's semantics: a test
// passes when the set of receivers actually reached equals the declared set.
func compare(spec *Spec, arrivals []arrival) *report {
	// Build observed receiver sets keyed by case name.
	observed := make(map[string]map[string]bool)
	for _, a := range arrivals {
		set, ok := observed[a.caseName]
		if !ok {
			set = make(map[string]bool)
			observed[a.caseName] = set
		}
		set[a.receiver] = true
	}

	rep := &report{Results: make([]caseResult, 0, len(spec.Tests))}

	for _, t := range spec.Tests {
		expected := toSet(t.ExpectedReceivers)
		actual := observed[t.Name] // nil map is fine: ranges as empty

		missing := difference(expected, actual)
		unexpected := difference(actual, expected)

		rep.Results = append(rep.Results, caseResult{
			Name:       t.Name,
			Expected:   sortedKeys(expected),
			Actual:     sortedKeys(actual),
			Missing:    missing,
			Unexpected: unexpected,
			Passed:     len(missing) == 0 && len(unexpected) == 0,
		})
	}

	return rep
}

func toSet(items []string) map[string]bool {
	set := make(map[string]bool, len(items))
	for _, it := range items {
		set[it] = true
	}
	return set
}

// difference returns the keys in a that are not in b, sorted.
func difference(a, b map[string]bool) []string {
	var diff []string
	for k := range a {
		if !b[k] {
			diff = append(diff, k)
		}
	}
	sort.Strings(diff)
	return diff
}

func sortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
