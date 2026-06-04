package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestCompare(t *testing.T) {
	spec := &Spec{Tests: []Test{
		{Name: "exact", ExpectedReceivers: []string{"a"}},
		{Name: "fanout", ExpectedReceivers: []string{"a", "b"}},
		{Name: "silenced", ExpectedReceivers: nil},
		{Name: "missing-one", ExpectedReceivers: []string{"a", "b"}},
		{Name: "unexpected-one", ExpectedReceivers: []string{"a"}},
	}}

	arrivals := []arrival{
		{caseName: "exact", receiver: "a"},
		{caseName: "fanout", receiver: "b"},
		{caseName: "fanout", receiver: "a"}, // order should not matter
		// "silenced" has no arrivals: expected empty set
		{caseName: "missing-one", receiver: "a"}, // b is missing
		{caseName: "unexpected-one", receiver: "a"},
		{caseName: "unexpected-one", receiver: "c"}, // c is unexpected
	}

	rep := compare(spec, arrivals)

	want := map[string]struct {
		passed     bool
		missing    []string
		unexpected []string
	}{
		"exact":          {passed: true},
		"fanout":         {passed: true},
		"silenced":       {passed: true},
		"missing-one":    {passed: false, missing: []string{"b"}},
		"unexpected-one": {passed: false, unexpected: []string{"c"}},
	}

	for _, r := range rep.Results {
		w := want[r.Name]
		if r.Passed != w.passed {
			t.Errorf("%s: passed = %v, want %v", r.Name, r.Passed, w.passed)
		}
		if len(w.missing) > 0 && !reflect.DeepEqual(r.Missing, w.missing) {
			t.Errorf("%s: missing = %v, want %v", r.Name, r.Missing, w.missing)
		}
		if len(w.unexpected) > 0 && !reflect.DeepEqual(r.Unexpected, w.unexpected) {
			t.Errorf("%s: unexpected = %v, want %v", r.Name, r.Unexpected, w.unexpected)
		}
	}

	if rep.allPassed() {
		t.Error("report should not pass overall")
	}
}

func TestReadArrivals(t *testing.T) {
	in := `{"receiver":"receiver-a","case":"c1"}

{"receiver":"receiver-b","case":"c1"}
{"receiver":"receiver-c","case":"c2"}`

	arrivals, err := readArrivals(strings.NewReader(in))
	if err != nil {
		t.Fatalf("readArrivals: %v", err)
	}

	want := []arrival{
		{caseName: "c1", receiver: "receiver-a"},
		{caseName: "c1", receiver: "receiver-b"},
		{caseName: "c2", receiver: "receiver-c"},
	}
	if !reflect.DeepEqual(arrivals, want) {
		t.Errorf("arrivals = %v, want %v", arrivals, want)
	}
}

func TestReadArrivalsRejectsMissingFields(t *testing.T) {
	cases := map[string]string{
		"missing case":     `{"receiver":"receiver-a"}`,
		"missing receiver": `{"case":"c1"}`,
		"not json":         `receiver-a c1`,
	}
	for name, in := range cases {
		if _, err := readArrivals(strings.NewReader(in)); err == nil {
			t.Errorf("%s: expected error, got nil", name)
		}
	}
}
