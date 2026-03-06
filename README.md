# go-test-regroup

`go-test-regroup` processes verbose `go test -v` output and reconstructs
interleaved parallel test logs into per-test output, preserving
parent/child test hierarchy.

## Why?

When Go runs tests in parallel, output from different tests can become
interleaved. `go-test-regroup` regroups that output so each test's logs appear
together, making failures and test-local diagnostics much easier to
read.

## Installation

```sh
GOPROXY=direct go install github.com/frobware/go-test-regroup@latest
```

The `GOPROXY=direct` is needed because the Go module proxy has a
cached entry from the repo's previous name that conflicts with
`@latest` resolution. This will resolve once the proxy cache expires.

To build from source with version information:

```sh
make build
```

## Usage

The simplest case is to pipe Go test output directly to `go-test-regroup`:

```sh
go test ./... -v | go-test-regroup
```

This regroups all interleaved parallel test output into a clean,
hierarchical format where each test's output is kept together.

You can also process existing log files or URLs:

```sh
go-test-regroup test.log
go-test-regroup https://path/to/test.log
```

### Viewing failures

```sh
go-test-regroup -l test.log     # Show failed test names
go-test-regroup -L test.log     # Show failed tests with their output
```

### Writing per-test files

`go-test-regroup -w` writes each test's regrouped output to
`<base>/<test-name>/output.log`. Subtests with `/` in their names
become nested directories.

```sh
go-test-regroup -w test.log              # Write to current directory
go-test-regroup -w -o out/ test.log      # Write to out/
go-test-regroup -w --reuse test.log      # Reuse existing directories
```

### Test filtering

The `-t` flag accepts a regular expression to filter which tests to
process. It can be combined with any output mode:

```sh
go-test-regroup -t "TestAuth.*" test.log        # Filter regrouped output
go-test-regroup -t "TestAuth.*" -l test.log     # Filter failure summary
go-test-regroup -t "TestAuth.*" -w test.log     # Filter file output
```

### Unmatched output

By default, lines that cannot be confidently attributed to a test are
suppressed. This includes preamble before the first test,
output from truncated in-flight tests, and trailing content after the
test run. Use `-u` to show them:

```sh
go-test-regroup -u test.log
```

When writing files with `-w`, unmatched lines are always written to
`_unmatched/output.log`.

## Notes

- `go-test-regroup` works on the observable verbose output format produced by
  `go test -v`. It does not require or use `go test -json`.
- Output that cannot be confidently attributed to a test is preserved
  as unmatched rather than guessed. Incomplete tests at EOF
  are marked with `=== INCOMPLETE TestName ===` to preserve their
  attribution context.

## Synopsis

```
go-test-regroup [options] [file|url ...]
  -F, --reuse
        Reuse existing output directories instead of failing
  -L    Print summary of failures and include the full output for each failure
  -d    Enable debug output
  -l    Print summary of failures (list test names with failures)
  -o string
        Base directory to write output files (default ".")
  -t string
        Regular expression to filter test names (default ".*")
  -u    Show unmatched lines that could not be attributed to a test
  -V, --version
        Print version information and exit
  -w    Write each test's output to individual files
```
