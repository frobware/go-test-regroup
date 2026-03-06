package main

import (
	"errors"
	"regexp"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		wantTests     []TestResult
		wantUnmatched []string
		wantErr       bool
	}{
		{
			name: "single passing test",
			input: `=== RUN   TestFoo
    foo_test.go:10: some log
--- PASS: TestFoo (0.01s)
PASS
`,
			wantTests: []TestResult{
				{Name: "TestFoo", Status: "PASS", Time: "0.01s", Level: 0, Output: []string{"    foo_test.go:10: some log"}},
			},
		},
		{
			name: "single failing test",
			input: `=== RUN   TestBar
    bar_test.go:5: expected 1 got 2
--- FAIL: TestBar (0.02s)
FAIL
`,
			wantTests: []TestResult{
				{Name: "TestBar", Status: "FAIL", Time: "0.02s", Level: 0, Output: []string{"    bar_test.go:5: expected 1 got 2"}},
			},
		},
		{
			name: "nested subtests",
			input: `=== RUN   TestParent
=== RUN   TestParent/child1
    parent_test.go:10: child1 log
=== RUN   TestParent/child2
    parent_test.go:15: child2 log
    --- PASS: TestParent/child1 (0.01s)
    --- PASS: TestParent/child2 (0.02s)
--- PASS: TestParent (0.05s)
PASS
`,
			wantTests: []TestResult{
				{Name: "TestParent/child1", Status: "PASS", Time: "0.01s", Level: 1, Output: []string{"    parent_test.go:10: child1 log"}},
				{Name: "TestParent/child2", Status: "PASS", Time: "0.02s", Level: 1, Output: []string{"    parent_test.go:15: child2 log"}},
				{Name: "TestParent", Status: "PASS", Time: "0.05s", Level: 0, Output: []string{}},
			},
		},
		{
			name: "parallel tests with PAUSE/CONT",
			input: `=== RUN   TestA
=== PAUSE TestA
=== RUN   TestB
=== PAUSE TestB
=== CONT  TestA
    a_test.go:5: log from A
=== CONT  TestB
    b_test.go:5: log from B
--- PASS: TestA (0.01s)
--- PASS: TestB (0.02s)
PASS
`,
			wantTests: []TestResult{
				{Name: "TestA", Status: "PASS", Time: "0.01s", Level: 0, Output: []string{"    a_test.go:5: log from A"}},
				{Name: "TestB", Status: "PASS", Time: "0.02s", Level: 0, Output: []string{"    b_test.go:5: log from B"}},
			},
		},
		{
			name: "preamble lines before first RUN",
			input: `some build output
another preamble line
=== RUN   TestOnly
    only_test.go:3: hello
--- PASS: TestOnly (0.00s)
PASS
`,
			wantTests: []TestResult{
				{Name: "TestOnly", Status: "PASS", Time: "0.00s", Level: 0, Output: []string{"    only_test.go:3: hello"}},
			},
			wantUnmatched: []string{"some build output", "another preamble line"},
		},
		{
			name:  "empty input",
			input: "",
		},
		{
			name: "NAME context switches to named test",
			input: `=== RUN   TestX
=== RUN   TestX/sub
=== PAUSE TestX/sub
=== CONT  TestX/sub
    x_test.go:10: sub output
=== NAME  TestX
    x_test.go:20: parent cleanup
--- PASS: TestX/sub (0.01s)
--- PASS: TestX (0.02s)
PASS
`,
			wantTests: []TestResult{
				{Name: "TestX/sub", Status: "PASS", Time: "0.01s", Level: 0, Output: []string{"    x_test.go:10: sub output"}},
				{Name: "TestX", Status: "PASS", Time: "0.02s", Level: 0, Output: []string{"    x_test.go:20: parent cleanup"}},
			},
		},
		{
			name: "multiple summary records mixed PASS and FAIL",
			input: `=== RUN   TestGood
    good_test.go:1: ok
=== RUN   TestBad
    bad_test.go:1: not ok
--- PASS: TestGood (0.01s)
--- FAIL: TestBad (0.02s)
FAIL
`,
			wantTests: []TestResult{
				{Name: "TestGood", Status: "PASS", Time: "0.01s", Level: 0, Output: []string{"    good_test.go:1: ok"}},
				{Name: "TestBad", Status: "FAIL", Time: "0.02s", Level: 0, Output: []string{"    bad_test.go:1: not ok"}},
			},
		},
		{
			name: "sequential top-level tests with output after summary",
			input: `=== RUN   TestFirst
    first_test.go:1: hello
--- PASS: TestFirst (0.01s)
=== RUN   TestSecond
    second_test.go:1: world
--- PASS: TestSecond (0.02s)
PASS
`,
			wantTests: []TestResult{
				{Name: "TestFirst", Status: "PASS", Time: "0.01s", Level: 0, Output: []string{"    first_test.go:1: hello"}},
				{Name: "TestSecond", Status: "PASS", Time: "0.02s", Level: 0, Output: []string{"    second_test.go:1: world"}},
			},
		},
		{
			name: "truncated input promotes in-flight buffers to unmatched",
			input: `=== RUN   TestComplete
    complete_test.go:1: done
--- PASS: TestComplete (0.01s)
=== RUN   TestIncomplete
    incomplete_test.go:1: started
    incomplete_test.go:2: still going
`,
			wantTests: []TestResult{
				{Name: "TestComplete", Status: "PASS", Time: "0.01s", Level: 0, Output: []string{"    complete_test.go:1: done"}},
			},
			wantUnmatched: []string{
				"=== INCOMPLETE TestIncomplete ===",
				"    incomplete_test.go:1: started",
				"    incomplete_test.go:2: still going",
			},
		},
		{
			name: "post-terminator content collected as unmatched",
			input: `=== RUN   TestOnly
    only_test.go:1: ok
--- PASS: TestOnly (0.01s)
PASS
exit status 0
some trailing junk
`,
			wantTests: []TestResult{
				{Name: "TestOnly", Status: "PASS", Time: "0.01s", Level: 0, Output: []string{"    only_test.go:1: ok"}},
			},
			wantUnmatched: []string{"exit status 0", "some trailing junk"},
		},
		{
			name: "truncated input with no completed tests",
			input: `=== RUN   TestNeverFinishes
    nf_test.go:1: line one
    nf_test.go:2: line two
`,
			wantUnmatched: []string{
				"=== INCOMPLETE TestNeverFinishes ===",
				"    nf_test.go:1: line one",
				"    nf_test.go:2: line two",
			},
		},
		// Note: "multiple incomplete tests at EOF" is tested
		// separately to verify deterministic lexical ordering
		// across multiple in-flight buffers.
		{
			name: "truncated subtest run preserves child and parent residue",
			input: `=== RUN   TestParent
=== RUN   TestParent/child1
    child1_test.go:1: child1 output
    --- PASS: TestParent/child1 (0.01s)
=== RUN   TestParent/child2
    child2_test.go:1: child2 started
`,
			wantTests: []TestResult{
				{Name: "TestParent/child1", Status: "PASS", Time: "0.01s", Level: 1, Output: []string{"    child1_test.go:1: child1 output"}},
			},
			wantUnmatched: []string{
				"=== INCOMPLETE TestParent/child2 ===",
				"    child2_test.go:1: child2 started",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Parse(strings.NewReader(tt.input))
			if (err != nil) != tt.wantErr {
				t.Fatalf("Parse() error = %v, wantErr %v", err, tt.wantErr)
			}

			if len(result.Tests) != len(tt.wantTests) {
				t.Fatalf("got %d tests, want %d\ngot:  %+v", len(result.Tests), len(tt.wantTests), result.Tests)
			}
			for i, want := range tt.wantTests {
				got := result.Tests[i]
				if got.Name != want.Name || got.Status != want.Status || got.Time != want.Time || got.Level != want.Level {
					t.Errorf("test[%d] metadata:\ngot:  {Name:%q Status:%q Time:%q Level:%d}\nwant: {Name:%q Status:%q Time:%q Level:%d}",
						i, got.Name, got.Status, got.Time, got.Level,
						want.Name, want.Status, want.Time, want.Level)
				}
				if len(got.Output) != len(want.Output) {
					t.Errorf("test[%d] %q: got %d output lines, want %d\ngot:  %v\nwant: %v",
						i, want.Name, len(got.Output), len(want.Output), got.Output, want.Output)
					continue
				}
				for j := range want.Output {
					if got.Output[j] != want.Output[j] {
						t.Errorf("test[%d] %q output[%d]:\ngot:  %q\nwant: %q", i, want.Name, j, got.Output[j], want.Output[j])
					}
				}
			}

			if len(result.Unmatched) != len(tt.wantUnmatched) {
				t.Errorf("got %d unmatched lines, want %d\ngot:  %v\nwant: %v",
					len(result.Unmatched), len(tt.wantUnmatched), result.Unmatched, tt.wantUnmatched)
			} else {
				for i, want := range tt.wantUnmatched {
					if result.Unmatched[i] != want {
						t.Errorf("unmatched[%d]:\ngot:  %q\nwant: %q", i, result.Unmatched[i], want)
					}
				}
			}
		})
	}
}

func TestParseStream(t *testing.T) {
	input := `preamble
=== RUN   TestA
    a.go:1: hello
=== RUN   TestB
    b.go:1: world
--- PASS: TestA (0.01s)
--- FAIL: TestB (0.02s)
FAIL
`
	var events []Event
	err := ParseStream(strings.NewReader(input), func(e Event) error {
		events = append(events, e)
		return nil
	})
	if err != nil {
		t.Fatalf("ParseStream() error = %v", err)
	}

	// Expect 3 events: TestA completion, TestB completion, unmatched.
	if len(events) != 3 {
		t.Fatalf("got %d events, want 3", len(events))
	}

	// First event: TestA completes.
	if events[0].Test == nil || events[0].Test.Name != "TestA" {
		t.Errorf("event[0]: expected TestA completion, got %+v", events[0])
	}

	// Second event: TestB completes.
	if events[1].Test == nil || events[1].Test.Name != "TestB" {
		t.Errorf("event[1]: expected TestB completion, got %+v", events[1])
	}

	// Third event: unmatched preamble.
	if events[2].Unmatched == nil || len(events[2].Unmatched) != 1 || events[2].Unmatched[0] != "preamble" {
		t.Errorf("event[2]: expected unmatched [preamble], got %+v", events[2])
	}
}

func TestParseStreamCallbackError(t *testing.T) {
	input := `=== RUN   TestA
--- PASS: TestA (0.01s)
PASS
`
	sentinel := errors.New("stop")
	err := ParseStream(strings.NewReader(input), func(e Event) error {
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Errorf("expected sentinel error, got %v", err)
	}
}

func TestFilter(t *testing.T) {
	result := &ParseResult{
		Tests: []TestResult{
			{Name: "TestAlpha", Status: "PASS", Output: []string{"line1"}},
			{Name: "TestBeta", Status: "FAIL", Output: []string{"line2"}},
			{Name: "TestGamma", Status: "PASS", Output: []string{"line3"}},
		},
		Unmatched: []string{"preamble"},
	}

	re := regexp.MustCompile(`Beta`)
	filtered := result.Filter(re)

	if len(filtered.Tests) != 1 {
		t.Fatalf("expected 1 test, got %d", len(filtered.Tests))
	}
	if filtered.Tests[0].Name != "TestBeta" {
		t.Errorf("expected TestBeta, got %s", filtered.Tests[0].Name)
	}
	if len(filtered.Unmatched) != 1 || filtered.Unmatched[0] != "preamble" {
		t.Errorf("expected unmatched preserved, got %v", filtered.Unmatched)
	}
}

func TestFilterIndependence(t *testing.T) {
	result := &ParseResult{
		Tests:     []TestResult{{Name: "TestA", Status: "PASS"}},
		Unmatched: []string{"original"},
	}

	re := regexp.MustCompile(`.*`)
	filtered := result.Filter(re)

	// Mutating the filtered result must not affect the original.
	filtered.Unmatched[0] = "modified"
	if result.Unmatched[0] != "original" {
		t.Errorf("Filter shares backing array: original unmatched was modified to %q", result.Unmatched[0])
	}
}

func TestMerge(t *testing.T) {
	a := &ParseResult{
		Tests: []TestResult{
			{Name: "TestOne", Status: "PASS", Output: []string{"a1"}},
		},
		Unmatched: []string{"preamble-a"},
	}
	b := &ParseResult{
		Tests: []TestResult{
			{Name: "TestTwo", Status: "FAIL", Output: []string{"b1"}},
		},
		Unmatched: []string{"preamble-b"},
	}

	a.Merge(b)

	if len(a.Tests) != 2 {
		t.Fatalf("expected 2 tests, got %d", len(a.Tests))
	}
	if a.Tests[0].Name != "TestOne" || a.Tests[1].Name != "TestTwo" {
		t.Errorf("expected [TestOne, TestTwo], got [%s, %s]", a.Tests[0].Name, a.Tests[1].Name)
	}
	if len(a.Unmatched) != 2 || a.Unmatched[0] != "preamble-a" || a.Unmatched[1] != "preamble-b" {
		t.Errorf("expected merged unmatched, got %v", a.Unmatched)
	}
}

func TestMultipleIncompleteTestsAtEOF(t *testing.T) {
	input := `=== RUN   TestAlpha
    alpha_test.go:1: alpha output
=== RUN   TestBeta
    beta_test.go:1: beta output
=== RUN   TestGamma
    gamma_test.go:1: gamma output
`
	result, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(result.Tests) != 0 {
		t.Errorf("expected 0 completed tests, got %d", len(result.Tests))
	}

	// Incomplete buffers are promoted in lexical order by test
	// name, so the output is deterministic.
	wantUnmatched := []string{
		"=== INCOMPLETE TestAlpha ===",
		"    alpha_test.go:1: alpha output",
		"=== INCOMPLETE TestBeta ===",
		"    beta_test.go:1: beta output",
		"=== INCOMPLETE TestGamma ===",
		"    gamma_test.go:1: gamma output",
	}

	if len(result.Unmatched) != len(wantUnmatched) {
		t.Fatalf("got %d unmatched lines, want %d\ngot:  %v\nwant: %v",
			len(result.Unmatched), len(wantUnmatched), result.Unmatched, wantUnmatched)
	}
	for i, want := range wantUnmatched {
		if result.Unmatched[i] != want {
			t.Errorf("unmatched[%d]:\ngot:  %q\nwant: %q", i, result.Unmatched[i], want)
		}
	}
}
