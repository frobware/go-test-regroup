package main

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
)

var (
	runRegex     = regexp.MustCompile(`^=== RUN\s+(.*)`)
	nameRegex    = regexp.MustCompile(`^=== NAME\s+(.*)`)
	contRegex    = regexp.MustCompile(`^=== CONT\s+(.*)`)
	summaryRegex = regexp.MustCompile(`^\s*--- (PASS|FAIL|SKIP): (\S+)\s+\(([^)]*)\)`)
)

// ParseStream reads Go test output from r line by line, emitting an
// Event each time a test completes (its --- PASS/FAIL/SKIP line is
// seen) or at the end for any unmatched lines. The emit callback
// receives events in completion order.
//
// At EOF, any buffered output for tests that never received a summary
// line is promoted to unmatched. Content appearing after a bare
// PASS/FAIL terminator is also collected as unmatched. Both policies
// preserve information rather than silently discarding it.
func ParseStream(r io.Reader, emit func(Event) error) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	buffers := make(map[string][]string)
	var unmatched []string
	var currentTest string
	started := false
	terminated := false

	for scanner.Scan() {
		line := scanner.Text()

		// After a bare terminator, collect remaining lines as
		// unmatched rather than discarding them.
		if terminated {
			unmatched = append(unmatched, line)
			continue
		}

		if !started {
			if runRegex.MatchString(line) {
				started = true
			} else {
				unmatched = append(unmatched, line)
				continue
			}
		}

		trimmed := strings.TrimSpace(line)

		// Bare PASS/FAIL terminator: stop parsing test structure
		// but continue scanning for post-terminator content.
		if trimmed == PassMarker || trimmed == FailMarker {
			terminated = true
			continue
		}

		// Summary line: --- PASS/FAIL/SKIP: TestName (duration)
		if m := summaryRegex.FindStringSubmatch(line); m != nil {
			indent := len(line) - len(strings.TrimLeft(line, " "))
			result := TestResult{
				Name:   m[2],
				Status: m[1],
				Time:   m[3],
				Level:  indent / 4,
				Output: buffers[m[2]],
			}
			delete(buffers, m[2])

			if err := emit(Event{Test: &result}); err != nil {
				return err
			}
			continue
		}

		switch {
		case strings.HasPrefix(trimmed, "=== PAUSE"):
			// nothing to do
		case contRegex.MatchString(line):
			match := contRegex.FindStringSubmatch(line)
			currentTest = match[1]
		case nameRegex.MatchString(line):
			match := nameRegex.FindStringSubmatch(line)
			currentTest = match[1]
		case runRegex.MatchString(line):
			match := runRegex.FindStringSubmatch(line)
			currentTest = match[1]
			if _, exists := buffers[currentTest]; !exists {
				buffers[currentTest] = []string{}
			}
		default:
			if currentTest != "" {
				buffers[currentTest] = append(buffers[currentTest], line)
			} else {
				unmatched = append(unmatched, line)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading input: %w", err)
	}

	// Promote in-flight buffers for tests that never completed
	// (e.g., truncated input) to unmatched output. Each test's
	// residue is prefixed with a synthetic marker so the
	// attribution context is not lost. Names are sorted for
	// deterministic output.
	names := make([]string, 0, len(buffers))
	for name := range buffers {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		lines := buffers[name]
		if len(lines) == 0 {
			continue
		}
		unmatched = append(unmatched, fmt.Sprintf("=== INCOMPLETE %s ===", name))
		unmatched = append(unmatched, lines...)
	}

	if len(unmatched) > 0 {
		if err := emit(Event{Unmatched: unmatched}); err != nil {
			return err
		}
	}

	return nil
}

// Parse reads Go test output from r and returns a ParseResult
// containing completed test results in completion order and any
// unmatched lines. It is a batch wrapper around ParseStream.
func Parse(r io.Reader) (*ParseResult, error) {
	result := &ParseResult{}
	err := ParseStream(r, func(e Event) error {
		if e.Test != nil {
			result.Tests = append(result.Tests, *e.Test)
		}
		if e.Unmatched != nil {
			result.Unmatched = append(result.Unmatched, e.Unmatched...)
		}
		return nil
	})
	return result, err
}
