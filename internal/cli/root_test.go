package cli

import (
	"bytes"
	"errors"
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
