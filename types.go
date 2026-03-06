package main

import "regexp"

const (
	FailMarker = "FAIL"
	PassMarker = "PASS"
)

type TestResult struct {
	Name   string
	Status string
	Time   string
	Level  int
	Output []string
}

type Event struct {
	Test      *TestResult
	Unmatched []string
}

type ParseResult struct {
	Tests     []TestResult
	Unmatched []string
}

// Filter returns a new ParseResult containing only tests whose
// names match re. Unmatched lines are preserved as-is.
func (r *ParseResult) Filter(re *regexp.Regexp) *ParseResult {
	filtered := &ParseResult{
		Unmatched: append([]string(nil), r.Unmatched...),
	}
	for _, t := range r.Tests {
		if re.MatchString(t.Name) {
			filtered.Tests = append(filtered.Tests, t)
		}
	}
	return filtered
}

// Merge incorporates the tests and unmatched lines from other
// into r, preserving the completion order of both.
func (r *ParseResult) Merge(other *ParseResult) {
	r.Tests = append(r.Tests, other.Tests...)
	r.Unmatched = append(r.Unmatched, other.Unmatched...)
}
