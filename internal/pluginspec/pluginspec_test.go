// Package pluginspec holds the structural tests for the Claude Code
// marketplace and plugin that ship from this repository. They stand in for
// `claude plugin validate` on machines and CI jobs where the Claude CLI is not
// available.
package pluginspec

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	repoRoot   = "../.."
	pluginPath = repoRoot + "/plugins/prayops"
)

func readJSON(t *testing.T, path string, into any) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if err := json.Unmarshal(raw, into); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
}

type marketplace struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Owner       struct {
		Name string `json:"name"`
	} `json:"owner"`
	Plugins []struct {
		Name        string `json:"name"`
		Source      string `json:"source"`
		Description string `json:"description"`
		Version     string `json:"version"`
	} `json:"plugins"`
}

type pluginManifest struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Repository  string `json:"repository"`
	License     string `json:"license"`
	Author      struct {
		Name string `json:"name"`
	} `json:"author"`
	Keywords []string `json:"keywords"`
}

type runtimeManifest struct {
	SchemaVersion  int    `json:"schemaVersion"`
	RuntimeVersion string `json:"runtimeVersion"`
	Repository     string `json:"repository"`
	AssetTemplate  string `json:"assetTemplate"`
	ChecksumAsset  string `json:"checksumAsset"`
	Supported      []struct {
		OS   string `json:"os"`
		Arch string `json:"arch"`
	} `json:"supported"`
}

func loadMarketplace(t *testing.T) marketplace {
	t.Helper()
	var m marketplace
	readJSON(t, repoRoot+"/.claude-plugin/marketplace.json", &m)
	return m
}

func loadPlugin(t *testing.T) pluginManifest {
	t.Helper()
	var p pluginManifest
	readJSON(t, pluginPath+"/.claude-plugin/plugin.json", &p)
	return p
}

func loadRuntime(t *testing.T) runtimeManifest {
	t.Helper()
	var r runtimeManifest
	readJSON(t, pluginPath+"/runtime-manifest.json", &r)
	return r
}

// FR-001: the marketplace entry must point at ./plugins/prayops with a
// relative source that stays inside the repository.
func TestMarketplaceSourceResolvesInsideRepository(t *testing.T) {
	m := loadMarketplace(t)

	if m.Name != "prayops" {
		t.Fatalf("marketplace name = %q, want \"prayops\"", m.Name)
	}
	if m.Owner.Name == "" || m.Description == "" || m.Version == "" {
		t.Fatalf("marketplace is missing owner, description, or version: %+v", m)
	}
	if len(m.Plugins) != 1 {
		t.Fatalf("marketplace declares %d plugins, want 1", len(m.Plugins))
	}

	entry := m.Plugins[0]
	if entry.Name != "prayops" {
		t.Fatalf("plugin entry name = %q, want \"prayops\"", entry.Name)
	}
	// The Claude Code marketplace schema requires an explicitly relative path
	// from the repository root. A bare "prayops" or "plugins/prayops" is
	// rejected by `claude plugin validate`.
	if !strings.HasPrefix(entry.Source, "./") {
		t.Fatalf("plugin source %q must start with \"./\"", entry.Source)
	}
	if filepath.IsAbs(entry.Source) || strings.Contains(entry.Source, "..") {
		t.Fatalf("plugin source %q must stay inside the repository", entry.Source)
	}

	resolved := filepath.Join(repoRoot, entry.Source)
	if _, err := os.Stat(filepath.Join(resolved, ".claude-plugin", "plugin.json")); err != nil {
		t.Fatalf("plugin source %q does not resolve to a plugin: %v", resolved, err)
	}
}

// FR-011 / D-034: marketplace, plugin, and runtime versions ship together.
func TestVersionsAreSynchronized(t *testing.T) {
	m := loadMarketplace(t)
	p := loadPlugin(t)
	r := loadRuntime(t)

	versions := map[string]string{
		"marketplace":       m.Version,
		"marketplace entry": m.Plugins[0].Version,
		"plugin manifest":   p.Version,
		"runtime manifest":  r.RuntimeVersion,
	}
	for label, got := range versions {
		if got != m.Version {
			t.Fatalf("%s version = %q, want %q", label, got, m.Version)
		}
	}

	if p.Name != "prayops" || p.DisplayName == "" || p.Description == "" ||
		p.Author.Name == "" || p.Repository == "" || p.License == "" || len(p.Keywords) == 0 {
		t.Fatalf("plugin manifest is missing required fields: %+v", p)
	}
}

// FR-023: the runtime manifest must describe exactly the assets GoReleaser
// produces, or first-use installation 404s.
func TestRuntimeManifestMatchesReleaseAssets(t *testing.T) {
	r := loadRuntime(t)

	if r.SchemaVersion != 1 {
		t.Fatalf("runtime manifest schemaVersion = %d, want 1", r.SchemaVersion)
	}
	if r.AssetTemplate != "prayops_{version}_{os}_{arch}.tar.gz" {
		t.Fatalf("assetTemplate = %q", r.AssetTemplate)
	}
	if r.ChecksumAsset != "checksums.txt" {
		t.Fatalf("checksumAsset = %q", r.ChecksumAsset)
	}

	want := map[string]bool{
		"darwin/arm64": false,
		"darwin/amd64": false,
		"linux/arm64":  false,
		"linux/amd64":  false,
	}
	for _, target := range r.Supported {
		key := target.OS + "/" + target.Arch
		if _, ok := want[key]; !ok {
			t.Fatalf("runtime manifest claims unsupported target %q", key)
		}
		want[key] = true
	}
	for key, seen := range want {
		if !seen {
			t.Fatalf("runtime manifest is missing target %q", key)
		}
	}

	raw, err := os.ReadFile(repoRoot + "/.goreleaser.yaml")
	if err != nil {
		t.Fatalf("read goreleaser config: %v", err)
	}
	config := string(raw)
	for _, needle := range []string{
		"prayops_{{ .Version }}_{{ .Os }}_{{ .Arch }}",
		"name_template: checksums.txt",
		"formats: [tar.gz]",
	} {
		if !strings.Contains(config, needle) {
			t.Fatalf("goreleaser config does not contain %q", needle)
		}
	}
}

// FR-030..FR-033 / D-008: the four skills exist, and none of them may be
// invoked by the model on its own.
func TestSkillFrontmatter(t *testing.T) {
	for _, name := range []string{"setup", "pray", "watch", "doctor"} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(pluginPath, "skills", name, "SKILL.md")
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read skill: %v", err)
			}

			fields := parseFrontmatter(t, string(raw), path)
			if fields["name"] != name {
				t.Fatalf("frontmatter name = %q, want %q", fields["name"], name)
			}
			if fields["description"] == "" {
				t.Fatal("frontmatter is missing description")
			}
			if fields["disable-model-invocation"] != "true" {
				t.Fatalf("disable-model-invocation = %q, want \"true\"", fields["disable-model-invocation"])
			}
		})
	}
}

func parseFrontmatter(t *testing.T, content, path string) map[string]string {
	t.Helper()
	if !strings.HasPrefix(content, "---\n") {
		t.Fatalf("%s does not open with YAML frontmatter", path)
	}
	end := strings.Index(content[4:], "\n---")
	if end < 0 {
		t.Fatalf("%s has an unterminated frontmatter block", path)
	}

	fields := map[string]string{}
	for _, line := range strings.Split(content[4:4+end], "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		fields[strings.TrimSpace(key)] = strings.Trim(strings.TrimSpace(value), `"'`)
	}
	return fields
}

type hooksFile struct {
	Hooks map[string][]struct {
		Matcher string `json:"matcher"`
		Hooks   []struct {
			Type    string `json:"type"`
			Command string `json:"command"`
			Timeout int    `json:"timeout"`
		} `json:"hooks"`
	} `json:"hooks"`
}

func loadHooks(t *testing.T) hooksFile {
	t.Helper()
	var h hooksFile
	readJSON(t, pluginPath+"/hooks/hooks.json", &h)
	return h
}

// shippedHookCommands returns one command per lifecycle event as declared in
// hooks.json.
func shippedHookCommands(t *testing.T) map[string]string {
	t.Helper()
	commands := map[string]string{}
	for event, matchers := range loadHooks(t).Hooks {
		for _, matcher := range matchers {
			for _, hook := range matcher.Hooks {
				commands[event] = hook.Command
			}
		}
	}
	if len(commands) == 0 {
		t.Fatal("hooks.json declares no commands")
	}
	return commands
}

// FR-060 / FR-061: every lifecycle event is wired, and every hook command is
// guarded so a missing runtime cannot fail the hook.
func TestHooksCoverLifecycleAndGuardMissingRuntime(t *testing.T) {
	hooks := loadHooks(t)

	for _, event := range []string{
		"SessionStart", "UserPromptSubmit", "PreToolUse", "PostToolUse",
		"PostToolUseFailure", "PermissionRequest", "Stop", "StopFailure", "SessionEnd",
	} {
		matchers, ok := hooks.Hooks[event]
		if !ok || len(matchers) == 0 {
			t.Fatalf("hooks.json does not handle %s", event)
		}
		records := false
		for _, matcher := range matchers {
			for _, hook := range matcher.Hooks {
				if hook.Type != "command" {
					t.Fatalf("%s hook type = %q, want \"command\"", event, hook.Type)
				}
				if hook.Timeout <= 0 {
					t.Fatalf("%s hook has no timeout", event)
				}
				// The runtime ships inside the plugin, so putting it in place
				// is a copy. Reaching the network from a hook still is not.
				if strings.Contains(hook.Command, "curl") || strings.Contains(hook.Command, "wget") {
					t.Fatalf("%s hook downloads something: %q", event, hook.Command)
				}
				if !strings.Contains(hook.Command, `${CLAUDE_PLUGIN_DATA}/bin/prayops`) {
					continue
				}
				records = true
				if !strings.Contains(hook.Command, `test ! -x`) {
					t.Fatalf("%s hook is not guarded against a missing runtime: %q", event, hook.Command)
				}
			}
		}
		if !records {
			t.Fatalf("no %s hook records an event", event)
		}
	}
}

// FR-027 / acceptance test 3: with no runtime installed, every shipped hook
// command must exit 0 and stay silent. The commands are read from hooks.json
// so this exercises what actually ships, not a copy of it.
func TestHookCommandsAreNoOpWithoutRuntime(t *testing.T) {
	for event, command := range shippedHookCommands(t) {
		t.Run(event, func(t *testing.T) {
			cmd := exec.Command("sh", "-c", command)
			cmd.Env = append(os.Environ(), "CLAUDE_PLUGIN_DATA="+t.TempDir())
			cmd.Stdin = strings.NewReader(`{"session_id":"s1","hook_event_name":"` + event + `"}`)

			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("hook failed without a runtime: %v (%q)", err, out)
			}
			if len(out) != 0 {
				t.Fatalf("hook produced output: %q", out)
			}
		})
	}
}

// FR-012 / D-011: installing the plugin is the whole installation. The
// launcher works out of the box because the binary shipped with it - there is
// no separate step to send the user off to, and nothing to download.
func TestLauncherRunsWithoutASeparateInstall(t *testing.T) {
	launcher, err := filepath.Abs(pluginPath + "/bin/prayops")
	if err != nil {
		t.Fatalf("resolve launcher: %v", err)
	}

	cmd := exec.Command(launcher, "version")
	// An empty data directory: the plugin is installed, no session has run.
	cmd.Env = append(os.Environ(), "CLAUDE_PLUGIN_DATA="+t.TempDir())

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("launcher failed with a fresh data directory: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), manifestRuntimeVersion(t)) {
		t.Fatalf("launcher ran a different version: %q", out)
	}
}

// The runtime shipped in the plugin is put in place by a copy, and a copy
// alone: a hook that reached the network would be installing code the user
// never agreed to.
func TestEnsureRuntimeInstallsByCopying(t *testing.T) {
	data := t.TempDir()

	script, err := filepath.Abs(pluginPath + "/scripts/ensure-runtime.sh")
	if err != nil {
		t.Fatalf("resolve script: %v", err)
	}
	root, err := filepath.Abs(pluginPath)
	if err != nil {
		t.Fatalf("resolve plugin root: %v", err)
	}

	raw, err := os.ReadFile(script)
	if err != nil {
		t.Fatalf("read script: %v", err)
	}
	for _, forbidden := range []string{"curl", "wget", "http://", "https://"} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("ensure-runtime.sh mentions %q", forbidden)
		}
	}

	run := func() string {
		t.Helper()
		cmd := exec.Command(script)
		cmd.Env = append(os.Environ(),
			"CLAUDE_PLUGIN_ROOT="+root, "CLAUDE_PLUGIN_DATA="+data)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("ensure-runtime: %v\n%s", err, out)
		}
		return string(out)
	}

	run()

	installed := filepath.Join(data, "bin", "prayops")
	info, err := os.Stat(installed)
	if err != nil {
		t.Fatalf("nothing was installed: %v", err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("the installed runtime is not executable (mode %v)", info.Mode().Perm())
	}

	out, err := exec.Command(installed, "version").CombinedOutput()
	if err != nil || !strings.Contains(string(out), manifestRuntimeVersion(t)) {
		t.Fatalf("installed runtime reports %q: %v", out, err)
	}

	// Running again is free: an unchanged version must not re-copy, because
	// this runs on every session start.
	before, err := os.Stat(installed)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	run()
	after, err := os.Stat(installed)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("an up-to-date runtime was copied again")
	}
}

func manifestRuntimeVersion(t *testing.T) string {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join(pluginPath, "runtime-manifest.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var manifest struct {
		RuntimeVersion string `json:"runtimeVersion"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	return manifest.RuntimeVersion
}

// FR-012: the plugin is copied into a cache on install, so nothing inside it
// may reach outside its own root.
func TestPluginIsSelfContained(t *testing.T) {
	err := filepath.Walk(pluginPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		// The shipped binaries are not text. Scanning them for path fragments
		// finds compiler leftovers, not a script reaching outside the plugin,
		// which is what this check is for.
		if strings.HasPrefix(filepath.ToSlash(strings.TrimPrefix(path, pluginPath+"/")), "runtime/") {
			return nil
		}

		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, escape := range []string{"../", "${CLAUDE_PLUGIN_ROOT}/../", "cmd/prayops", "go run"} {
			if strings.Contains(string(raw), escape) {
				t.Errorf("%s references %q outside the plugin root", path, escape)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk plugin: %v", err)
	}
}

// The launcher and bootstrap scripts are executed directly, so the executable
// bit has to survive into the repository.
func TestShippedScriptsAreExecutable(t *testing.T) {
	for _, rel := range []string{
		"bin/prayops",
		"scripts/setup.sh",
		"scripts/ensure-runtime.sh",
	} {
		info, err := os.Stat(filepath.Join(pluginPath, rel))
		if err != nil {
			t.Fatalf("stat %s: %v", rel, err)
		}
		if info.Mode().Perm()&0o111 == 0 {
			t.Fatalf("%s is not executable (mode %v)", rel, info.Mode().Perm())
		}
	}
}

// Skills run their commands through the Bash tool, which does not get the
// environment a hook gets. A skill that reaches for CLAUDE_PROJECT_DIR or
// CLAUDE_SESSION_ID silently passes an empty string; one that builds a path
// from CLAUDE_PLUGIN_DATA gets another plugin's directory, which is how
// doctor came to report a runtime it had just installed as missing.
func TestSkillsOnlyUseVariablesTheBashToolHas(t *testing.T) {
	// CLAUDE_PLUGIN_ROOT and CLAUDE_PLUGIN_DATA are substituted into skill
	// text by Claude Code, so they are fine to write; the rest have to exist
	// in the shell that runs the command.
	banned := map[string]string{
		"CLAUDE_SESSION_ID":  "CLAUDE_CODE_SESSION_ID",
		"CLAUDE_PROJECT_DIR": "$PWD",
	}

	skills, err := filepath.Glob(filepath.Join(pluginPath, "skills", "*", "SKILL.md"))
	if err != nil || len(skills) == 0 {
		t.Fatalf("no skills found: %v", err)
	}

	for _, skill := range skills {
		raw, err := os.ReadFile(skill)
		if err != nil {
			t.Fatalf("read %s: %v", skill, err)
		}
		body := string(raw)

		for name, instead := range banned {
			// CLAUDE_SESSION_ID is a prefix of nothing, but CLAUDE_PROJECT_DIR
			// and the allowed names share none either, so a plain search is
			// enough as long as the longer name is checked first.
			if strings.Contains(strings.ReplaceAll(body, "CLAUDE_CODE_SESSION_ID", ""), name) {
				t.Errorf("%s uses %s, which the Bash tool does not set; use %s",
					filepath.Base(filepath.Dir(skill)), name, instead)
			}
		}

		// The runtime is reached through the launcher, which resolves the data
		// directory itself.
		if strings.Contains(body, "${CLAUDE_PLUGIN_DATA}/bin/prayops") {
			t.Errorf("%s builds a runtime path from CLAUDE_PLUGIN_DATA; use ${CLAUDE_PLUGIN_ROOT}/bin/prayops",
				filepath.Base(filepath.Dir(skill)))
		}
	}
}

// dataLayout builds Claude Code's own <root>/plugins/data/<market>-<plugin>
// arrangement: ours, and one belonging to somebody else.
func dataLayout(t *testing.T) (ours, theirs string) {
	t.Helper()

	base := filepath.Join(t.TempDir(), "plugins", "data")
	ours = filepath.Join(base, "prayops-prayops")
	theirs = filepath.Join(base, "codex-openai-codex")
	for _, dir := range []string{ours, theirs} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	return ours, theirs
}

func absPluginRoot(t *testing.T) string {
	t.Helper()

	root, err := filepath.Abs(pluginPath)
	if err != nil {
		t.Fatalf("resolve plugin root: %v", err)
	}
	return root
}

// A skill's command runs in the Bash tool, where CLAUDE_PLUGIN_DATA is
// another plugin's directory. Installing there does not just miss - it writes
// a runtime.json that makes the wrong directory look like a real install from
// then on, and both the launcher and the Go side believe it.
func TestEnsureRuntimeRefusesAnotherPluginsDirectory(t *testing.T) {
	ours, theirs := dataLayout(t)
	root := absPluginRoot(t)

	cmd := exec.Command(filepath.Join(root, "scripts", "ensure-runtime.sh"))
	cmd.Env = append(os.Environ(),
		"CLAUDE_PLUGIN_ROOT="+root, "CLAUDE_PLUGIN_DATA="+theirs)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("ensure-runtime: %v\n%s", err, out)
	}

	for _, name := range []string{"bin/prayops", "runtime.json"} {
		if _, err := os.Stat(filepath.Join(theirs, name)); err == nil {
			t.Fatalf("wrote %s into another plugin's directory", name)
		}
	}
	if _, err := os.Stat(filepath.Join(ours, "bin", "prayops")); err != nil {
		t.Fatalf("did not install into our own directory: %v", err)
	}
}

// The very first session has not copied anything yet, so the launcher runs
// the binary shipped inside the plugin - which sits at runtime/<os>_<arch>
// and cannot work out the data directory from its own location. If the
// launcher forwards the inherited value, that binary writes a whole session's
// state into whichever plugin the environment happened to name.
func TestLauncherNeverPassesOnAnotherPluginsDirectory(t *testing.T) {
	ours, theirs := dataLayout(t)
	root := absPluginRoot(t)

	cmd := exec.Command(filepath.Join(root, "bin", "prayops"), "pray", "--cwd", "/x")
	cmd.Env = append(os.Environ(), "CLAUDE_PLUGIN_DATA="+theirs)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("pray: %v\n%s", err, out)
	}

	if entries, err := os.ReadDir(theirs); err != nil || len(entries) != 0 {
		t.Fatalf("another plugin's directory holds %d entries", len(entries))
	}
	if _, err := os.Stat(filepath.Join(ours, "state")); err != nil {
		t.Fatalf("the prayer did not land in our own directory: %v", err)
	}
}

// The headline of this release is that an update needs nothing but
// `/plugin update`. That only holds if a stale runtime is actually replaced.
func TestEnsureRuntimeReplacesAStaleRuntime(t *testing.T) {
	ours, _ := dataLayout(t)
	root := absPluginRoot(t)

	run := func() {
		t.Helper()
		cmd := exec.Command(filepath.Join(root, "scripts", "ensure-runtime.sh"))
		cmd.Env = append(os.Environ(),
			"CLAUDE_PLUGIN_ROOT="+root, "CLAUDE_PLUGIN_DATA="+ours)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("ensure-runtime: %v\n%s", err, out)
		}
	}

	run()

	// What an older release left behind: a runtime.json naming a version that
	// is no longer what the plugin ships.
	record := filepath.Join(ours, "runtime.json")
	if err := os.WriteFile(record, []byte(`{"runtimeVersion":"0.0.1"}`), 0o644); err != nil {
		t.Fatalf("write record: %v", err)
	}
	installed := filepath.Join(ours, "bin", "prayops")
	if err := os.WriteFile(installed, []byte("#!/bin/sh\nexit 9\n"), 0o755); err != nil {
		t.Fatalf("write stale binary: %v", err)
	}

	run()

	out, err := exec.Command(installed, "version").CombinedOutput()
	if err != nil || !strings.Contains(string(out), manifestRuntimeVersion(t)) {
		t.Fatalf("the stale runtime was not replaced: %q (%v)", out, err)
	}
	raw, err := os.ReadFile(record)
	if err != nil || !strings.Contains(string(raw), manifestRuntimeVersion(t)) {
		t.Fatalf("runtime.json still reads %q", raw)
	}
}

// An update has to land without restarting the session.
//
// Most plugins are markdown and hooks.json, so /plugin update is the whole
// update. This one carries a binary that has to reach the path the status line
// runs, and it used to get there only at the next session start - so the
// update reported success while the old version went on running.
//
// The replacement happens from inside the old binary, on the next hook, which
// is the next tool call.
func TestAHookReplacesAStaleRuntime(t *testing.T) {
	root := absPluginRoot(t)
	data := t.TempDir()

	if err := os.MkdirAll(filepath.Join(data, "bin"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// A runtime from some older release, standing where the status line looks.
	stale := filepath.Join(data, "bin", "prayops")
	build := exec.Command("go", "build", "-ldflags", "-X main.version=0.0.1", "-o", stale, "./cmd/prayops")
	build.Dir = repoRoot
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}

	hook := exec.Command(stale, "hook", "claude")
	hook.Env = append(os.Environ(), "CLAUDE_PLUGIN_ROOT="+root, "CLAUDE_PLUGIN_DATA="+data)
	hook.Stdin = strings.NewReader(
		`{"session_id":"s1","cwd":"/x","hook_event_name":"PreToolUse","tool_name":"Bash"}`)
	if out, err := hook.CombinedOutput(); err != nil || len(out) != 0 {
		t.Fatalf("hook: %v, output %q", err, out)
	}

	want := manifestRuntimeVersion(t)
	out, err := exec.Command(stale, "version").CombinedOutput()
	if err != nil || !strings.Contains(string(out), want) {
		t.Fatalf("after one hook the runtime reports %q, want %s", out, want)
	}

	raw, err := os.ReadFile(filepath.Join(data, "runtime.json"))
	if err != nil || !strings.Contains(string(raw), want) {
		t.Fatalf("runtime.json still reads %q", raw)
	}
}

// And it must not copy on every hook when there is nothing to replace: this
// runs on every tool call.
func TestAHookLeavesACurrentRuntimeAlone(t *testing.T) {
	root := absPluginRoot(t)
	data := t.TempDir()

	cmd := exec.Command(filepath.Join(root, "scripts", "ensure-runtime.sh"))
	cmd.Env = append(os.Environ(), "CLAUDE_PLUGIN_ROOT="+root, "CLAUDE_PLUGIN_DATA="+data)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("ensure-runtime: %v\n%s", err, out)
	}

	installed := filepath.Join(data, "bin", "prayops")
	before, err := os.Stat(installed)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	hook := exec.Command(installed, "hook", "claude")
	hook.Env = append(os.Environ(), "CLAUDE_PLUGIN_ROOT="+root, "CLAUDE_PLUGIN_DATA="+data)
	hook.Stdin = strings.NewReader(
		`{"session_id":"s1","cwd":"/x","hook_event_name":"PreToolUse","tool_name":"Bash"}`)
	if out, err := hook.CombinedOutput(); err != nil {
		t.Fatalf("hook: %v\n%s", err, out)
	}

	after, err := os.Stat(installed)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("a hook rewrote a runtime that was already current")
	}
}
