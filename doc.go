/*
Package main implements go-test-regroup, a tool for reconstructing interleaved
Go test output into per-test logical groupings.

# Go test output format

The parser targets the observable verbose output format produced by
the testing package (go test -v). The following structural line forms
are recognised:

	=== RUN   TestName       test starts
	=== PAUSE TestName       test calls t.Parallel(), pauses
	=== CONT  TestName       parallel test resumes execution
	=== NAME  TestName       output context switches to TestName
	--- PASS: TestName (d)   test passed
	--- FAIL: TestName (d)   test failed
	--- SKIP: TestName (d)   test skipped
	PASS                     bare terminator, all tests passed
	FAIL                     bare terminator, one or more tests failed

There is no bare SKIP terminator; only PASS and FAIL appear as
top-level terminators.

# Completion semantics

A test's output is sealed by its --- PASS/FAIL/SKIP summary line.
Once the testing package prints that line, t.done is set and no
further output is routed through the chatty printer for that test
name. This makes the summary line a safe point at which to emit or
discard the test's buffered output.

For subtests, the parent's summary line is guaranteed to appear after
all children. The testing package's tRunner blocks on each child's
signal channel before calling report() on the parent. This produces
a strict ordering: children complete before parent.

# Context switching

The === NAME line is not always emitted. It appears only when the
chatty printer's internal lastName state differs from the test
currently producing output, typically during interleaved parallel
execution. It is authoritative: the chatty printer emits it using
the name field of the test that is actually writing, so when it
says === NAME TestParent, the output genuinely comes from the
parent, not from a child. The parser trusts it unconditionally and
redirects subsequent lines to the named test.

# Attribution policy

Lines that cannot be confidently attributed to a specific test are
collected as unmatched output rather than being silently dropped or
speculatively attached to the most recently active test. This
includes preamble lines before the first === RUN, orphaned output
with no active test context, and any other content that falls outside
the recognised structural markers.

At EOF, any buffered output for tests that never received a summary
line (e.g., because the input was truncated) is promoted to unmatched.
Each such group is prefixed with a synthetic marker of the form
"=== INCOMPLETE TestName ===" so the in-flight attribution context is
not lost. Content appearing after a bare PASS/FAIL terminator is also
collected as unmatched, without a marker since it had no test
attribution. Both policies preserve information rather than silently
discarding it.

The CLI suppresses unmatched output by default; the -u flag reveals
it.

# Design constraints

The parser targets the observable output contract, not internal
implementation details of the testing package. The assumptions above
were validated against Go 1.25 src/testing/testing.go but are
intentionally limited to behaviours that are stable across releases:
the set of structural line forms, the seal-point guarantee, and the
parent-after-children ordering. Changes to these would break
cmd/test2json and a wide ecosystem of tooling.
*/
package main
