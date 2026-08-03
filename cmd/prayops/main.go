// Command prayops is the PrayOps runtime.
//
// Implemented: version and doctor for the bootstrap installer, hook for
// recording lifecycle events, statusline for the compact Claude Code status
// line and its settings entry, and alias for the optional /pray skill.
//
// The watch, pray, and setup commands are reserved and exit non-zero rather
// than pretending to work, so no caller can mistake a stub for working
// behaviour.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/cruellaDev/claude-code-prayops/contracts"
	"github.com/cruellaDev/claude-code-prayops/internal/doctor"
	"github.com/cruellaDev/claude-code-prayops/internal/hook"
	"github.com/cruellaDev/claude-code-prayops/internal/session"
	"github.com/cruellaDev/claude-code-prayops/internal/spool"
	"github.com/cruellaDev/claude-code-prayops/internal/statusline"
	"github.com/cruellaDev/claude-code-prayops/internal/userconfig"
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
		return runStatusline(args[1:], stdin, stdout, stderr)
	case "alias":
		return runAlias(args[1:], stdout, stderr)
	case "watch", "pray", "setup":
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
func runStatusline(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		switch args[0] {
		case "install", "uninstall", "status":
			return runStatuslineConfig(args, stdout, stderr)
		}
	}

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

// runStatuslineConfig installs, removes, or reports the user's status line
// setting.
//
// Like the bootstrap, install without --yes prints what it would change and
// exits 10, because the caller is usually a skill with no terminal to prompt
// from.
func runStatuslineConfig(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("statusline "+args[0], flag.ContinueOnError)
	fs.SetOutput(stderr)
	assumeYes := fs.Bool("yes", false, "apply the change")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}

	data := os.Getenv("CLAUDE_PLUGIN_DATA")
	if data == "" {
		fmt.Fprintln(stderr, "prayops: CLAUDE_PLUGIN_DATA is not set")
		return 1
	}
	settingsPath := userconfig.SettingsPath()
	if settingsPath == "" {
		fmt.Fprintln(stderr, "prayops: could not locate the Claude Code settings file")
		return 1
	}

	manager := userconfig.NewManager(settingsPath, data)
	line := userconfig.NewStatusLine(filepath.Join(data, "bin", "prayops"))

	switch args[0] {
	case "status":
		record, err := manager.LoadRecord()
		if err != nil {
			fmt.Fprintf(stderr, "prayops: %v\n", err)
			return 1
		}
		current, present, err := manager.Current()
		if err != nil {
			fmt.Fprintf(stderr, "prayops: %v\n", err)
			return 1
		}
		switch {
		case record.Installed:
			fmt.Fprintf(stdout, "Status line  Installed by PrayOps  %s\n", settingsPath)
		case present:
			fmt.Fprintf(stdout, "Status line  Configured by you     %s\n", settingsPath)
			fmt.Fprintf(stdout, "             %s\n", current)
		default:
			fmt.Fprintf(stdout, "Status line  Not configured        %s\n", settingsPath)
		}
		return 0

	case "install":
		current, present, err := manager.Current()
		if err != nil {
			fmt.Fprintf(stderr, "prayops: %v\n", err)
			return 1
		}
		if !*assumeYes {
			fmt.Fprintf(stdout, "PrayOps status line\n\n  Settings  %s\n  Command   %s\n", settingsPath, line.Command)
			if present {
				fmt.Fprintf(stdout, "  Replaces  %s\n", current)
				fmt.Fprintf(stdout, "\nThe whole settings file is copied first and the value above is\nrestored by `prayops statusline uninstall`.\n")
			}
			fmt.Fprintln(stdout, "\nNothing has been changed. Re-run with --yes to confirm.")
			return 10
		}

		record, err := manager.InstallStatusLine(line)
		if err != nil {
			fmt.Fprintf(stderr, "prayops: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "Status line installed in %s\n", settingsPath)
		if record.BackupPath != "" {
			fmt.Fprintf(stdout, "Previous settings backed up to %s\n", record.BackupPath)
		}
		return 0

	default: // uninstall
		if !*assumeYes {
			fmt.Fprintf(stdout, "Would restore the previous status line in %s.\n", settingsPath)
			fmt.Fprintln(stdout, "\nNothing has been changed. Re-run with --yes to confirm.")
			return 10
		}
		changed, err := manager.UninstallStatusLine()
		if err != nil {
			fmt.Fprintf(stderr, "prayops: %v\n", err)
			return 1
		}
		if !changed {
			fmt.Fprintln(stdout, "No PrayOps status line to remove; your settings are unchanged.")
			return 0
		}
		fmt.Fprintf(stdout, "Status line removed from %s\n", settingsPath)
		return 0
	}
}

// runAlias installs or removes the optional user-scope /pray skill.
//
// The alias is a convenience only: /prayops:pray works without it, which is
// why a conflict is a refusal rather than something to resolve.
func runAlias(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "prayops: alias needs install, uninstall, or status")
		return 2
	}

	fs := flag.NewFlagSet("alias "+args[0], flag.ContinueOnError)
	fs.SetOutput(stderr)
	assumeYes := fs.Bool("yes", false, "apply the change")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}

	data := os.Getenv("CLAUDE_PLUGIN_DATA")
	if data == "" {
		fmt.Fprintln(stderr, "prayops: CLAUDE_PLUGIN_DATA is not set")
		return 1
	}
	configDir := userconfig.ConfigDir()
	if configDir == "" {
		fmt.Fprintln(stderr, "prayops: could not locate the Claude Code configuration directory")
		return 1
	}

	manager := userconfig.NewManager(userconfig.SettingsPath(), data)
	path := userconfig.AliasPath(configDir)

	switch args[0] {
	case "status":
		record, err := manager.LoadAliasRecord()
		if err != nil {
			fmt.Fprintf(stderr, "prayops: %v\n", err)
			return 1
		}
		_, statErr := os.Stat(path)
		switch {
		case record.Installed && statErr == nil:
			fmt.Fprintf(stdout, "Alias /pray  Installed by PrayOps  %s\n", path)
		case statErr == nil:
			fmt.Fprintf(stdout, "Alias /pray  Provided by another skill  %s\n", path)
		default:
			fmt.Fprintf(stdout, "Alias /pray  Not installed  %s\n", path)
		}
		return 0

	case "install":
		if !*assumeYes {
			fmt.Fprintf(stdout, "PrayOps /pray alias\n\n  Writes    %s\n  Runs      %s\n", path, filepath.Join(data, "bin", "prayops"))
			if _, err := os.Stat(path); err == nil {
				fmt.Fprintf(stdout, "\nSomething already provides /pray at that path. PrayOps will not\nreplace it. /prayops:pray works without the alias.\n")
			}
			fmt.Fprintln(stdout, "\nNothing has been changed. Re-run with --yes to confirm.")
			return 10
		}

		record, err := manager.InstallAlias(configDir, filepath.Join(data, "bin", "prayops"))
		var conflict userconfig.ErrAliasConflict
		if errors.As(err, &conflict) {
			fmt.Fprintf(stderr, "prayops: %s already exists and was left untouched.\n", conflict.Path)
			fmt.Fprintln(stderr, "Use /prayops:pray, or remove that file yourself first.")
			return 3
		}
		if err != nil {
			fmt.Fprintf(stderr, "prayops: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "Alias installed at %s\n", record.Path)
		return 0

	case "uninstall":
		if !*assumeYes {
			fmt.Fprintf(stdout, "Would remove %s.\n\nNothing has been changed. Re-run with --yes to confirm.\n", path)
			return 10
		}
		removed, err := manager.UninstallAlias()
		if err != nil {
			fmt.Fprintf(stderr, "prayops: %v\n", err)
			return 1
		}
		if !removed {
			fmt.Fprintln(stdout, "No PrayOps alias to remove; nothing was changed.")
			return 0
		}
		fmt.Fprintf(stdout, "Alias removed from %s\n", path)
		return 0

	default:
		fmt.Fprintf(stderr, "prayops: unknown alias command %q\n", args[0])
		return 2
	}
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
	pluginRoot := fs.String("plugin-root", "", "plugin root directory")
	pluginData := fs.String("plugin-data", "", "plugin data directory")
	asJSON := fs.Bool("json", false, "print the report as JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	// The bootstrap runs this against a binary that has no plugin around it
	// yet, so the smoke test must not depend on any of the checks below.
	if *smoke {
		fmt.Fprintln(stdout, "ok")
		return 0
	}

	report := doctor.Run(doctor.Options{
		PluginRoot: firstNonEmpty(*pluginRoot, os.Getenv("CLAUDE_PLUGIN_ROOT")),
		PluginData: firstNonEmpty(*pluginData, os.Getenv("CLAUDE_PLUGIN_DATA")),
		Settings:   userconfig.SettingsPath(),
	})

	if *asJSON {
		payload, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "prayops: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "%s\n", payload)
		return 0
	}

	fmt.Fprintf(stdout, "%-11s  %-4s  %s/%s\n", "Binary", doctor.StatusOK, runtime.GOOS, runtime.GOARCH)
	fmt.Fprint(stdout, report)
	if report.Failed() {
		return 1
	}
	return 0
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func usage(stderr io.Writer) {
	fmt.Fprint(stderr, `prayops - PrayOps runtime

Usage:
  prayops version [--json]
  prayops doctor [--bootstrap-smoke]
  prayops hook <host>
  prayops statusline <host>
  prayops statusline install|uninstall|status [--yes]
  prayops alias install|uninstall|status [--yes]
`)
}
