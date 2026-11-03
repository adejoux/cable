package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func run(args ...string) (string, error) {
	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func TestVersionCommand(t *testing.T) {
	old := version
	t.Cleanup(func() { version = old })
	version = "v1.2.3"
	out, err := run("version")
	if err != nil || strings.TrimSpace(out) != "cable v1.2.3" {
		t.Fatalf("version: %q, %v", out, err)
	}
	version = ""
	if got := Version(); got == "" {
		t.Fatal("Version must never be empty")
	}
}

func TestGlobalFlags(t *testing.T) {
	cmd := NewRootCommand()
	for name, short := range map[string]string{"collection": "c", "profile": "p"} {
		f := cmd.PersistentFlags().Lookup(name)
		if f == nil || f.Shorthand != short {
			t.Fatalf("missing global flag --%s/-%s", name, short)
		}
	}
	if _, err := run("-c", "api.cable.yaml", "-p", "dev", "version"); err != nil {
		t.Fatalf("global flags must be accepted by subcommands: %v", err)
	}
}

func TestRootWithoutTUIFails(t *testing.T) {
	if _, err := run(); !errors.Is(err, errNoTUI) {
		t.Fatalf("err = %v, want errNoTUI", err)
	}
	if _, err := run("extra"); err == nil {
		t.Fatal("unknown arguments must fail")
	}
}

func writeCollection(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "cable.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestListAndValidate(t *testing.T) {
	path := writeCollection(t, `version: 1
requests:
  - folder: auth
    requests:
      - id: login
        method: POST
        url: "{{ host }}/oauth/token"
  - id: health
    method: GET
    url: https://example.com/health
`)
	out, err := run("-c", path, "list")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"RECIPE", "auth.login  POST", "health      GET"} {
		if !strings.Contains(out, want) {
			t.Fatalf("list output misses %q:\n%s", want, out)
		}
	}
	out, err = run("-c", path, "validate")
	if err != nil || !strings.Contains(out, "ok, 2 recipes") {
		t.Fatalf("validate: %q, %v", out, err)
	}
}

func TestValidateReportsProblemsWithLines(t *testing.T) {
	path := writeCollection(t, "version: 1\nrequests:\n  - id: a\n    method: get\n")
	out, err := run("-c", path, "validate")
	if !errors.Is(err, errInvalid) {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(out, path+":3: recipe a: method \"get\" must be upper case") || !strings.Contains(out, "url is missing") {
		t.Fatalf("output:\n%s", out)
	}
}

func TestCollectionIsFoundFromTheWorkingDirectory(t *testing.T) {
	path := writeCollection(t, "version: 1\nrequests:\n  - {id: ping, method: GET, url: http://x}\n")
	sub := filepath.Join(filepath.Dir(path), "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(sub)
	out, err := run("list")
	if err != nil || !strings.Contains(out, "ping") {
		t.Fatalf("list from a subdirectory: %q, %v", out, err)
	}
	t.Chdir(t.TempDir())
	if _, err := run("list"); err == nil || !strings.Contains(err.Error(), "use -c") {
		t.Fatalf("err = %v", err)
	}
}
