package main

import (
	"fmt"
	"runtime"
	"strings"
)

// These variables are set at build time via -ldflags -X.
var (
	gitCommit string // full git commit hash
	gitBranch string // git branch name
	gitState  string // "clean" or "dirty"
	buildDate string // ISO 8601 build timestamp
	version   string // semantic version tag, if any
)

func versionString() string {
	v := version
	if v == "" {
		v = "(devel)"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Version:    %s\n", v)
	if gitCommit != "" {
		fmt.Fprintf(&b, "Git commit: %s\n", gitCommit)
	}
	if gitBranch != "" {
		fmt.Fprintf(&b, "Git branch: %s\n", gitBranch)
	}
	if gitState != "" {
		fmt.Fprintf(&b, "Git state:  %s\n", gitState)
	}
	if buildDate != "" {
		fmt.Fprintf(&b, "Build date: %s\n", buildDate)
	}
	fmt.Fprintf(&b, "Go version: %s\n", runtime.Version())
	fmt.Fprintf(&b, "Platform:   %s/%s\n", runtime.GOOS, runtime.GOARCH)
	return b.String()
}
