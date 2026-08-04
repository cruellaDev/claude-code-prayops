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
		for _, matcher := range matchers {
			for _, hook := range matcher.Hooks {
				if hook.Type != "command" {
					t.Fatalf("%s hook type = %q, want \"command\"", event, hook.Type)
				}
				if hook.Timeout <= 0 {
					t.Fatalf("%s hook has no timeout", event)
				}
				if !strings.Contains(hook.Command, `${CLAUDE_PLUGIN_DATA}/bin/prayops`) {
					t.Fatalf("%s hook does not call the installed runtime: %q", event, hook.Command)
				}
				if !strings.Contains(hook.Command, `test ! -x`) {
					t.Fatalf("%s hook is not guarded against a missing runtime: %q", event, hook.Command)
				}
				if strings.Contains(hook.Command, "setup") || strings.Contains(hook.Command, "curl") {
					t.Fatalf("%s hook must never install anything: %q", event, hook.Command)
				}
			}
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

// FR-012 / D-011: the launcher on the Bash tool PATH points users at setup
// instead of downloading anything itself.
func TestLauncherRefusesWithoutRuntime(t *testing.T) {
	launcher, err := filepath.Abs(pluginPath + "/bin/prayops")
	if err != nil {
		t.Fatalf("resolve launcher: %v", err)
	}

	cmd := exec.Command(launcher, "version")
	cmd.Env = append(os.Environ(), "CLAUDE_PLUGIN_DATA="+t.TempDir())

	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("launcher succeeded without a runtime: %q", out)
	}
	if !strings.Contains(string(out), "/prayops:setup") {
		t.Fatalf("launcher does not point at setup: %q", out)
	}
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
