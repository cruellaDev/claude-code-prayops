// Command prayops is the PrayOps runtime.
//
// This is the SP-02 release spike: only the commands the bootstrap installer
// needs to verify a freshly downloaded binary are implemented. Every other
// command is reserved and exits non-zero so no caller can mistake a stub for
// working behaviour.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
)

// version is injected at release time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return 2
	}

	switch args[0] {
	case "version", "--version", "-v":
		return runVersion(args[1:], stdout, stderr)
	case "doctor":
		return runDoctor(args[1:], stdout, stderr)
	case "hook":
		// Hooks must never block Claude Code. Until HOK-02 lands, drain stdin
		// so the host is not left writing into a closed pipe, emit nothing,
		// and succeed.
		_, _ = io.Copy(io.Discard, stdin)
		return 0
	case "watch", "pray", "statusline", "setup", "alias":
		fmt.Fprintf(stderr, "prayops: %q is not implemented in this build\n", args[0])
		return 2
	default:
		fmt.Fprintf(stderr, "prayops: unknown command %q\n", args[0])
		usage(stderr)
		return 2
	}
}

func runVersion(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("version", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "print machine readable version information")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if !*asJSON {
		fmt.Fprintf(stdout, "prayops %s\n", version)
		return 0
	}

	payload, err := json.Marshal(struct {
		Version string `json:"version"`
		OS      string `json:"os"`
		Arch    string `json:"arch"`
	}{version, runtime.GOOS, runtime.GOARCH})
	if err != nil {
		fmt.Fprintf(stderr, "prayops: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "%s\n", payload)
	return 0
}

func runDoctor(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(stderr)
	smoke := fs.Bool("bootstrap-smoke", false, "verify a freshly installed binary can execute")
	// Accepted for forward compatibility with the setup script; real
	// diagnostics against these paths arrive with CLD-04.
	fs.String("plugin-root", "", "plugin root directory")
	fs.String("plugin-data", "", "plugin data directory")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *smoke {
		fmt.Fprintln(stdout, "ok")
		return 0
	}

	fmt.Fprintf(stdout, "Runtime      OK  %s\n", version)
	fmt.Fprintf(stdout, "Platform     OK  %s/%s\n", runtime.GOOS, runtime.GOARCH)
	return 0
}

func usage(stderr io.Writer) {
	fmt.Fprint(stderr, `prayops - PrayOps runtime

Usage:
  prayops version [--json]
  prayops doctor [--bootstrap-smoke]
  prayops hook <host>
`)
}
