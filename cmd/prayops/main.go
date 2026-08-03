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
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/cruellaDev/claude-code-prayops/contracts"
	"github.com/cruellaDev/claude-code-prayops/internal/hook"
	"github.com/cruellaDev/claude-code-prayops/internal/session"
	"github.com/cruellaDev/claude-code-prayops/internal/spool"
	"github.com/cruellaDev/claude-code-prayops/internal/statusline"
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
		return runHook(args[1:], stdin)
	case "statusline":
		return runStatusline(args[1:], stdin, stdout)
	case "watch", "pray", "setup", "alias":
		fmt.Fprintf(stderr, "prayops: %q is not implemented in this build\n", args[0])
		return 2
	default:
		fmt.Fprintf(stderr, "prayops: unknown command %q\n", args[0])
		usage(stderr)
		return 2
	}
}

// runHook records one lifecycle event.
//
// It always exits 0 and always stays silent. A hook that fails loudly would
// interrupt the user's session over a decorative status display, so failures
// are recorded for doctor instead of reported.
func runHook(args []string, stdin io.Reader) int {
	host := contracts.HostClaude
	if len(args) > 0 && args[0] == string(contracts.HostCodex) {
		host = contracts.HostCodex
	}

	event, err := hook.NewAdapter(host).Adapt(stdin)
	// Drain whatever is left so Claude Code is never writing into a closed
	// pipe, which would surface as a hook error on its side.
	_, _ = io.Copy(io.Discard, stdin)
	if err != nil {
		recordHookError(err)
		return 0
	}

	data := os.Getenv("CLAUDE_PLUGIN_DATA")
	if data == "" {
		recordHookError(spool.ErrNoPluginData)
		return 0
	}
	stateDir := filepath.Join(data, "state")

	// The spool feeds the watcher's animation; the session file feeds the
	// status line. Both are written here because the hook is the only writer
	// guaranteed to run - a status line that depended on the watcher would
	// show nothing whenever no watcher is attached.
	if err := spool.New(stateDir).Write(event); err != nil {
		recordHookError(err)
	}
	if _, err := session.NewSessions(stateDir).Record(event); err != nil {
		recordHookError(err)
	}
	return 0
}

// statuslineInput is the allowlisted view of the status line payload. Claude
// Code sends model, workspace, and cost details that PrayOps has no use for.
type statuslineInput struct {
	SessionID string `json:"session_id"`
}

// runStatusline prints one compact line describing the caller's own session.
//
// Claude Code re-runs this on its refresh interval, so it must be a cheap
// snapshot: it reads one small file and never animates.
func runStatusline(args []string, stdin io.Reader, stdout io.Writer) int {
	fs := flag.NewFlagSet("statusline", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		return 0
	}

	var in statuslineInput
	if raw, err := io.ReadAll(io.LimitReader(stdin, hook.MaxInput)); err == nil {
		_ = json.Unmarshal(raw, &in)
	}

	data := os.Getenv("CLAUDE_PLUGIN_DATA")
	if data == "" {
		// Nothing to report and nowhere to complain to. An empty line beats
		// repeating an error on every refresh.
		return 0
	}

	state, _, err := session.NewSessions(filepath.Join(data, "state")).Load(in.SessionID)
	if err != nil {
		return 0
	}

	fmt.Fprintln(stdout, statusline.Render(state, statusline.Options{
		Now:     time.Now(),
		Columns: terminalColumns(),
		Color:   os.Getenv("NO_COLOR") == "",
	}))
	return 0
}

func terminalColumns() int {
	columns, err := strconv.Atoi(os.Getenv("COLUMNS"))
	if err != nil || columns <= 0 {
		return statusline.DefaultColumns
	}
	return columns
}

// recordHookError overwrites a single file, so it is self-bounding and needs
// no rate limiting. Nothing here is written to stdout or stderr.
func recordHookError(cause error) {
	data := os.Getenv("CLAUDE_PLUGIN_DATA")
	if data == "" {
		return
	}
	dir := filepath.Join(data, "state")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return
	}
	line := fmt.Sprintf("%s %v\n", time.Now().UTC().Format(time.RFC3339), cause)
	_ = os.WriteFile(filepath.Join(dir, "hook-last-error.txt"), []byte(line), 0o600)
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
  prayops statusline <host>
`)
}
