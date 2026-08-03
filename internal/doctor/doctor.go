// Package doctor reports what PrayOps can actually see on this machine.
//
// Every check reads; none of them install, download, or change configuration.
// A check that cannot determine an answer says so rather than guessing.
package doctor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cruellaDev/claude-code-prayops/internal/userconfig"
)

// Status is a check outcome.
type Status string

const (
	StatusOK      Status = "OK"
	StatusWarn    Status = "WARN"
	StatusFail    Status = "FAIL"
	StatusUnknown Status = "-"
)

// Check is one line of the report.
type Check struct {
	Name   string `json:"name"`
	Status Status `json:"status"`
	Detail string `json:"detail,omitempty"`
	Repair string `json:"repair,omitempty"`
}

// Report is the whole diagnosis.
type Report struct {
	Checks []Check `json:"checks"`
}

// Options are the paths and clock the checks run against.
type Options struct {
	PluginRoot string
	PluginData string
	Settings   string
	Now        time.Time
	Env        func(string) string
}

func (o Options) env(key string) string {
	if o.Env != nil {
		return o.Env(key)
	}
	return os.Getenv(key)
}

func (o Options) now() time.Time {
	if o.Now.IsZero() {
		return time.Now()
	}
	return o.Now
}

// Run performs every check.
func Run(opts Options) Report {
	pluginVersion, manifestVersion := readPluginVersions(opts.PluginRoot)
	installed := readInstalledVersion(opts.PluginData)

	return Report{Checks: []Check{
		checkPlugin(opts.PluginRoot, pluginVersion),
		checkRuntime(opts.PluginData, installed),
		checkVersions(manifestVersion, installed),
		checkState(opts.PluginData),
		checkHooks(opts.PluginData, opts.now()),
		checkStatusLine(opts),
		checkAlias(opts),
		checkWatcher(opts.PluginData, opts.now()),
		checkTerminal(opts),
	}}
}

// String renders the report as the aligned table the design calls for.
func (r Report) String() string {
	width := 0
	for _, check := range r.Checks {
		if len(check.Name) > width {
			width = len(check.Name)
		}
	}

	var b strings.Builder
	for _, check := range r.Checks {
		fmt.Fprintf(&b, "%-*s  %-4s  %s\n", width, check.Name, check.Status, check.Detail)
		if check.Repair != "" {
			fmt.Fprintf(&b, "%-*s        %s\n", width, "", check.Repair)
		}
	}
	return b.String()
}

// Failed reports whether anything needs the user's attention.
func (r Report) Failed() bool {
	for _, check := range r.Checks {
		if check.Status == StatusFail {
			return true
		}
	}
	return false
}

// oneLineJSON keeps a value the user formatted across several lines from
// breaking the table.
func oneLineJSON(raw json.RawMessage) string {
	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err != nil {
		return strings.Join(strings.Fields(string(raw)), " ")
	}
	return buf.String()
}

func jsonString(path, key string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return ""
	}
	var value string
	if err := json.Unmarshal(fields[key], &value); err != nil {
		return ""
	}
	return value
}

func readPluginVersions(root string) (plugin, manifest string) {
	if root == "" {
		return "", ""
	}
	return jsonString(filepath.Join(root, ".claude-plugin", "plugin.json"), "version"),
		jsonString(filepath.Join(root, "runtime-manifest.json"), "runtimeVersion")
}

func readInstalledVersion(data string) string {
	if data == "" {
		return ""
	}
	return jsonString(filepath.Join(data, "runtime.json"), "runtimeVersion")
}

func checkPlugin(root, version string) Check {
	switch {
	case root == "":
		return Check{"Plugin", StatusUnknown, "plugin root unknown",
			"run this from Claude Code, or pass --plugin-root"}
	case version == "":
		return Check{"Plugin", StatusFail, root, "plugin.json is missing or unreadable"}
	default:
		return Check{"Plugin", StatusOK, version, ""}
	}
}

func checkRuntime(data, installed string) Check {
	if data == "" {
		return Check{"Runtime", StatusUnknown, "plugin data unknown",
			"run this from Claude Code, or pass --plugin-data"}
	}

	binary := filepath.Join(data, "bin", "prayops")
	info, err := os.Stat(binary)
	switch {
	case err != nil:
		return Check{"Runtime", StatusFail, "not installed", "run /prayops:setup"}
	case info.Mode().Perm()&0o111 == 0:
		return Check{"Runtime", StatusFail, "not executable", "run /prayops:setup to reinstall"}
	case installed == "":
		return Check{"Runtime", StatusWarn, "installed, but runtime.json is missing",
			"run /prayops:setup to record the installation"}
	default:
		return Check{"Runtime", StatusOK, installed, ""}
	}
}

func checkVersions(manifest, installed string) Check {
	switch {
	case manifest == "" || installed == "":
		return Check{"Version", StatusUnknown, "nothing to compare", ""}
	case manifest == installed:
		return Check{"Version", StatusOK, installed, ""}
	default:
		return Check{"Version", StatusWarn,
			fmt.Sprintf("plugin expects %s, installed %s", manifest, installed),
			"run /prayops:setup to update the runtime"}
	}
}

func countFiles(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	return len(entries)
}

func checkState(data string) Check {
	if data == "" {
		return Check{"State", StatusUnknown, "plugin data unknown", ""}
	}

	events := filepath.Join(data, "state", "events")
	if _, err := os.Stat(events); err != nil {
		return Check{"State", StatusWarn, "no event spool yet", "it is created by the first hook"}
	}

	pending := countFiles(filepath.Join(events, "inbox"))
	rejected := countFiles(filepath.Join(events, "rejected"))

	detail := fmt.Sprintf("%d pending", pending)
	if rejected > 0 {
		return Check{"State", StatusWarn,
			fmt.Sprintf("%s, %d rejected", detail, rejected),
			"rejected events are quarantined in state/events/rejected"}
	}
	return Check{"State", StatusOK, detail, ""}
}

// checkHooks cannot read Claude Code's own configuration, so it reports
// evidence instead: whether a hook has ever recorded anything here.
func checkHooks(data string, now time.Time) Check {
	if data == "" {
		return Check{"Hooks", StatusUnknown, "plugin data unknown", ""}
	}

	if raw, err := os.ReadFile(filepath.Join(data, "state", "hook-last-error.txt")); err == nil {
		return Check{"Hooks", StatusWarn, strings.TrimSpace(string(raw)),
			"the last hook did not record its event"}
	}

	newest, ok := newestSessionUpdate(data)
	if !ok {
		return Check{"Hooks", StatusWarn, "no events recorded yet",
			"start a Claude Code turn, then run this again"}
	}
	return Check{"Hooks", StatusOK, "last event " + humanAge(now.Sub(newest)), ""}
}

func newestSessionUpdate(data string) (time.Time, bool) {
	dir := filepath.Join(data, "state", "sessions")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return time.Time{}, false
	}

	var newest time.Time
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(newest) {
			newest = info.ModTime()
		}
	}
	return newest, !newest.IsZero()
}

func humanAge(age time.Duration) string {
	switch {
	case age < time.Minute:
		return "just now"
	case age < time.Hour:
		return fmt.Sprintf("%dm ago", int(age.Minutes()))
	case age < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(age.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(age.Hours()/24))
	}
}

func checkStatusLine(opts Options) Check {
	if opts.PluginData == "" || opts.Settings == "" {
		return Check{"Status line", StatusUnknown, "settings location unknown", ""}
	}

	manager := userconfig.NewManager(opts.Settings, opts.PluginData)
	record, err := manager.LoadRecord()
	if err != nil {
		return Check{"Status line", StatusWarn, err.Error(), ""}
	}
	current, present, err := manager.Current()
	if err != nil {
		return Check{"Status line", StatusWarn, err.Error(),
			"fix the JSON in " + opts.Settings}
	}

	switch {
	case record.Installed && present:
		return Check{"Status line", StatusOK, "installed by PrayOps", ""}
	case record.Installed && !present:
		return Check{"Status line", StatusWarn, "recorded as installed but missing from settings",
			"run `prayops statusline install --yes` again"}
	case present:
		return Check{"Status line", StatusOK, "configured by you", oneLineJSON(current)}
	default:
		return Check{"Status line", StatusOK, "not configured",
			"optional: `prayops statusline install`"}
	}
}

// aliasPaths are where an opt-in /pray alias would live.
func aliasPaths(configDir string) []string {
	return []string{
		filepath.Join(configDir, "skills", "pray", "SKILL.md"),
		filepath.Join(configDir, "commands", "pray.md"),
	}
}

func checkAlias(opts Options) Check {
	configDir := opts.env("CLAUDE_CONFIG_DIR")
	if configDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return Check{"Alias /pray", StatusUnknown, "home directory unknown", ""}
		}
		configDir = filepath.Join(home, ".claude")
	}

	for _, path := range aliasPaths(configDir) {
		if _, err := os.Stat(path); err == nil {
			return Check{"Alias /pray", StatusOK, path, ""}
		}
	}
	return Check{"Alias /pray", StatusOK, "not installed", "optional: /prayops:pray works without it"}
}

// checkWatcher reports whether a `prayops watch` has published a heartbeat
// recently. A watcher that is not running is not an error.
func checkWatcher(data string, now time.Time) Check {
	if data == "" {
		return Check{"Watcher", StatusUnknown, "plugin data unknown", ""}
	}

	dir := filepath.Join(data, "state", "watchers")
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) == 0 {
		return Check{"Watcher", StatusOK, "not running", "run `prayops watch` in another terminal"}
	}

	var newest time.Time
	for _, entry := range entries {
		info, err := entry.Info()
		if err == nil && info.ModTime().After(newest) {
			newest = info.ModTime()
		}
	}
	if now.Sub(newest) > time.Minute {
		return Check{"Watcher", StatusOK, "not running", "last heartbeat " + humanAge(now.Sub(newest))}
	}
	return Check{"Watcher", StatusOK, "running", ""}
}

func checkTerminal(opts Options) Check {
	if opts.env("NO_COLOR") != "" {
		return Check{"Terminal", StatusOK, "colour disabled by NO_COLOR", ""}
	}
	switch {
	case strings.Contains(opts.env("COLORTERM"), "truecolor"), strings.Contains(opts.env("COLORTERM"), "24bit"):
		return Check{"Terminal", StatusOK, "truecolor", ""}
	case strings.Contains(opts.env("TERM"), "256color"):
		return Check{"Terminal", StatusOK, "256 colours", "prayer images fall back to the ANSI-256 palette"}
	case opts.env("TERM") == "":
		return Check{"Terminal", StatusUnknown, "TERM is not set", ""}
	default:
		return Check{"Terminal", StatusOK, opts.env("TERM"), "prayer images fall back to a monochrome ramp"}
	}
}
