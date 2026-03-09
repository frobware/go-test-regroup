// go-test-regroup processes verbose go test -v output and reconstructs
// interleaved parallel test logs into per-test output, preserving
// parent/child test hierarchy. See doc.go for parser design details
// and README.md for full usage documentation.
package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// openInputs calls fn with a named reader for each positional
// argument (file path or URL), or stdin if none are given. Files and
// HTTP response bodies are closed after fn returns.
func openInputs(args []string, debug bool, fn func(name string, r io.Reader)) {
	if len(args) == 0 {
		if debug {
			fmt.Println("[DEBUG] No arguments provided, reading from stdin.")
		}
		fn("stdin", os.Stdin)
		return
	}
	for _, input := range args {
		if u, err := url.ParseRequestURI(input); err == nil && (u.Scheme == "http" || u.Scheme == "https") {
			if debug {
				fmt.Printf("[DEBUG] Fetching URL: %s\n", input)
			}
			resp, err := http.Get(input)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error fetching URL %s: %v\n", input, err)
				continue
			}
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				fmt.Fprintf(os.Stderr, "Error fetching URL %s: HTTP %d %s\n", input, resp.StatusCode, resp.Status)
				resp.Body.Close()
				continue
			}
			fn(input, resp.Body)
			resp.Body.Close()
		} else {
			if debug {
				fmt.Printf("[DEBUG] Opening file: %s\n", input)
			}
			file, err := os.Open(input)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error opening file %s: %v\n", input, err)
				continue
			}
			fn(input, file)
			file.Close()
		}
	}
}

// writeLines creates filePath and writes each line followed by a
// newline. Write and close errors are returned.
func writeLines(filePath string, lines []string) error {
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("creating file %s: %w", filePath, err)
	}
	var writeErr error
	for _, line := range lines {
		if _, err := file.WriteString(line + "\n"); err != nil {
			writeErr = err
			break
		}
	}
	if err := file.Close(); err != nil && writeErr == nil {
		writeErr = err
	}
	if writeErr != nil {
		return fmt.Errorf("writing %s: %w", filePath, writeErr)
	}
	return nil
}

// writeResults writes each test's output to an individual file under
// outputDir, organised by test name. Unmatched lines are written to
// _unmatched/output.log.
func writeResults(result *ParseResult, outputDir string, reuse, debug bool) error {
	for _, tr := range result.Tests {
		dirPath := filepath.Join(outputDir, tr.Name)

		if !reuse {
			if _, err := os.Stat(dirPath); err == nil {
				return fmt.Errorf("directory '%s' already exists", dirPath)
			}
		}

		if err := os.MkdirAll(dirPath, 0755); err != nil {
			return fmt.Errorf("creating directory %s: %w", dirPath, err)
		}
		if debug {
			fmt.Printf("[DEBUG] Created directory: %s\n", dirPath)
		}

		filePath := filepath.Join(dirPath, "output.log")
		if debug {
			fmt.Printf("[DEBUG] Writing to file: %s\n", filePath)
		}
		if err := writeLines(filePath, tr.Output); err != nil {
			return err
		}
	}
	if len(result.Unmatched) > 0 {
		dirPath := filepath.Join(outputDir, "_unmatched")
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			return fmt.Errorf("creating directory %s: %w", dirPath, err)
		}
		if err := writeLines(filepath.Join(dirPath, "output.log"), result.Unmatched); err != nil {
			return err
		}
	}
	return nil
}

// printResults writes test results to stdout. When failuresOnly is
// set, only failing tests are shown. When includeOutput is set, the
// full test output is included beneath each summary line. When
// showUnmatched is set, unmatched lines are appended at the end.
func printResults(result *ParseResult, failuresOnly, includeOutput, showUnmatched bool) {
	for _, tr := range result.Tests {
		if failuresOnly && tr.Status != FailMarker {
			continue
		}
		indent := strings.Repeat("    ", tr.Level)
		fmt.Printf("%s--- %s: %s (%s)\n", indent, tr.Status, tr.Name, tr.Time)
		showOutput := !failuresOnly || includeOutput
		if showOutput {
			for _, line := range tr.Output {
				fmt.Printf("%s    %s\n", indent, line)
			}
		}
	}
	if showUnmatched && len(result.Unmatched) > 0 {
		fmt.Println("=== UNMATCHED ===")
		for _, line := range result.Unmatched {
			fmt.Println(line)
		}
	}
}

func main() {
	var showVersion bool
	flag.BoolVar(&showVersion, "V", false, "Print version information and exit")
	flag.BoolVar(&showVersion, "version", false, "Print version information and exit")
	listFailures := flag.Bool("l", false, "Print summary of failures (list test names with failures)")
	listFailuresWithOutput := flag.Bool("L", false, "Print summary of failures and include the full output for each failure")
	var reuseFlag bool
	flag.BoolVar(&reuseFlag, "F", false, "Reuse existing output directories instead of failing")
	flag.BoolVar(&reuseFlag, "reuse", false, "Reuse existing output directories instead of failing")
	debugFlag := flag.Bool("d", false, "Enable debug output")
	showUnmatched := flag.Bool("u", false, "Show unmatched lines that could not be attributed to a test")
	writeFiles := flag.Bool("w", false, "Write each test's output to individual files")
	outputDir := flag.String("o", ".", "Base directory to write output files (default current directory)")
	testPattern := flag.String("t", ".*", "Regular expression to filter test names for summary output")

	flag.Parse()

	if showVersion {
		fmt.Print(versionString())
		return
	}

	reTest, err := regexp.Compile(*testPattern)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid regular expression for -t: %v\n", err)
		os.Exit(1)
	}

	if *writeFiles {
		merged := &ParseResult{}
		openInputs(flag.Args(), *debugFlag, func(name string, r io.Reader) {
			result, err := Parse(r)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error parsing %s: %v\n", name, err)
				return
			}
			merged.Merge(result)
		})
		filtered := merged.Filter(reTest)
		if err := writeResults(filtered, *outputDir, reuseFlag, *debugFlag); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	failuresOnly := *listFailures || *listFailuresWithOutput
	showOutput := !failuresOnly || *listFailuresWithOutput

	openInputs(flag.Args(), *debugFlag, func(name string, r io.Reader) {
		err := ParseStream(r, func(e Event) error {
			if e.Test != nil {
				if !reTest.MatchString(e.Test.Name) {
					return nil
				}
				if failuresOnly && e.Test.Status != FailMarker {
					return nil
				}
				indent := strings.Repeat("    ", e.Test.Level)
				fmt.Printf("%s--- %s: %s (%s)\n",
					indent, e.Test.Status, e.Test.Name, e.Test.Time)
				if showOutput {
					for _, line := range e.Test.Output {
						fmt.Printf("%s    %s\n", indent, line)
					}
				}
			}
			if e.Unmatched != nil && *showUnmatched {
				fmt.Println("=== UNMATCHED ===")
				for _, line := range e.Unmatched {
					fmt.Println(line)
				}
			}
			return nil
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing %s: %v\n", name, err)
		}
	})
}
