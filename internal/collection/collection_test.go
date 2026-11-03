package collection

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadSpecExample(t *testing.T) {
	c, err := Load("testdata/cable.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if problems := c.Validate(); len(problems) != 0 {
		t.Fatalf("the spec example must be valid, got %v", problems)
	}
	var paths []string
	for _, r := range c.Recipes() {
		paths = append(paths, r.Path)
	}
	if got := strings.Join(paths, ","); got != "auth.login,invoices.list,invoices.create" {
		t.Fatalf("recipe paths = %s", got)
	}
	create, ok := c.Recipe("invoices.create")
	if !ok || create.Method != "POST" || create.Body.Kind() != "json" || create.Line == 0 {
		t.Fatalf("invoices.create = %+v", create)
	}
	if c.Defaults.Timeout.Std() != 30*time.Second {
		t.Fatalf("timeout = %v", c.Defaults.Timeout.Std())
	}
	if !c.Profiles["prod"].Confirm || c.Secrets["password"].Remember != RememberSession {
		t.Fatal("profiles and secrets must be decoded")
	}
	if c.Requests[1].Folder.Auth.Scheme() != "bearer" {
		t.Fatal("folder auth must be decoded")
	}
}

func parseErr(t *testing.T, src string) string {
	t.Helper()
	_, err := Parse([]byte(src))
	if err == nil {
		t.Fatalf("expected a parse error for:\n%s", src)
	}
	return err.Error()
}

func problems(t *testing.T, src string) string {
	t.Helper()
	c, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var out []string
	for _, p := range c.Validate() {
		out = append(out, p.String())
	}
	return strings.Join(out, "\n")
}

const head = "version: 1\nrequests:\n"

func TestUnknownKeysSuggestTheClosest(t *testing.T) {
	msg := parseErr(t, head+"  - id: a\n    method: GET\n    urll: http://x\n")
	if !strings.Contains(msg, `unknown recipe key "urll"`) || !strings.Contains(msg, `did you mean "url"`) || !strings.Contains(msg, "line 5") {
		t.Fatalf("message = %s", msg)
	}
	msg = parseErr(t, "version: 1\nprofils: {}\nrequests: []\n")
	if !strings.Contains(msg, "profils") {
		t.Fatalf("top-level unknown keys must fail: %s", msg)
	}
	msg = parseErr(t, head+"  - folder: f\n    requets: []\n")
	if !strings.Contains(msg, `did you mean "requests"`) {
		t.Fatalf("message = %s", msg)
	}
}

func TestUnquotedTemplateGetsAHint(t *testing.T) {
	msg := parseErr(t, head+"  - id: a\n    method: GET\n    url: {{ host }}/v1\n")
	if !strings.Contains(msg, "must be quoted") {
		t.Fatalf("message = %s", msg)
	}
	if msg := parseErr(t, ""); !strings.Contains(msg, "empty") {
		t.Fatalf("empty file: %s", msg)
	}
}

func TestSecretsRules(t *testing.T) {
	got := problems(t, `version: 1
secrets:
  token:
    from: [file]
    value: abc
  api:
    from: []
  other:
    from: [vault, "env:", "cmd: "]
    remember: forever
requests:
  - id: a
    method: GET
    url: http://x
`)
	for _, want := range []string{
		"secret token: values are not allowed",
		"cable secrets set token",
		"secret api: from is empty",
		`unknown provider "vault"`,
		"env: needs a variable name",
		"cmd: needs a command",
		"remember must be send or session",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

func TestNamesAreIdentifiersAndNotReserved(t *testing.T) {
	got := problems(t, `version: 1
vars:
  response: x
  9lives: y
profiles:
  secret: {}
requests:
  - folder: env
    requests:
      - id: "bad id"
        method: GET
        url: http://x
`)
	for _, want := range []string{
		`variable name "response" is reserved`,
		`variable name "9lives" must start with a letter`,
		`profile name "secret" is reserved`,
		`folder name "env" is reserved`,
		`recipe name "bad id" must start`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

func TestRecipeRules(t *testing.T) {
	got := problems(t, `version: 1
requests:
  - folder: a
    requests:
      - id: x
        method: get
        url: http://x
      - id: x
        method: GET
        url: http://x
        body:
          json: {a: 1}
          text: hello
      - id: y
        url: " "
        trigger: always
        body: {}
      - method: GET
        url: http://x
  - folder: a
    requests: []
`)
	for _, want := range []string{
		`method "get" must be upper case`,
		"recipe a.x is already defined at line 5",
		"body has several kinds (json, text)",
		"recipe a.y: method is missing",
		"recipe a.y: url is missing",
		"trigger must be auto",
		"body needs one of json",
		"recipe without id",
		"folder a is already defined",
		"folder a has no requests",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "line 0") {
		t.Errorf("recipe problems must carry a line:\n%s", got)
	}
}

func TestAuthRules(t *testing.T) {
	got := problems(t, `version: 1
defaults:
  auth:
    bearer: ""
requests:
  - folder: f
    auth: none
    requests:
      - id: a
        method: GET
        url: http://x
        auth:
          basic: {password: p}
          api_key: {name: k, value: v}
      - id: b
        method: GET
        url: http://x
        auth:
          oidc: {grant: implicit}
      - id: c
        method: GET
        url: http://x
        auth:
          api_key: {name: k, value: v, in: cookie}
`)
	for _, want := range []string{
		"defaults: bearer auth needs a token",
		"auth has several schemes (basic, api_key)",
		"oidc auth needs issuer",
		"oidc auth needs client_id",
		"oidc grant must be one of",
		"api_key in must be header or query",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if msg := parseErr(t, head+"  - id: a\n    method: GET\n    url: x\n    auth: basic\n"); !strings.Contains(msg, "auth must be none or a mapping") {
		t.Fatalf("message = %s", msg)
	}
	if msg := parseErr(t, head+"  - id: a\n    method: GET\n    url: x\n    auth:\n      beare: t\n"); !strings.Contains(msg, `did you mean "bearer"`) {
		t.Fatalf("message = %s", msg)
	}
}

func TestVersionPromptsAndEmptyCollection(t *testing.T) {
	got := problems(t, "prompts:\n  c:\n    default: z\n    choices: [a, b]\n")
	for _, want := range []string{"version is missing", "default \"z\" is not one of its choices", "no requests"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if got := problems(t, "version: 2\nrequests:\n  - {id: a, method: GET, url: x}\n"); !strings.Contains(got, "version 2 is not supported") {
		t.Fatalf("got %s", got)
	}
	if msg := parseErr(t, head+"  - id: a\n    method: GET\n    url: x\n    timeout: soon\n"); !strings.Contains(msg, "invalid duration") {
		t.Fatalf("message = %s", msg)
	}
}

func TestFindWalksUpAndCollectionFileNames(t *testing.T) {
	root := t.TempDir()
	deep := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Find(deep); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
	want := filepath.Join(root, "a", DefaultFile)
	if err := os.WriteFile(want, []byte("version: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err := Find(deep); err != nil || got != want {
		t.Fatalf("Find = %q, %v", got, err)
	}
	for name, ok := range map[string]bool{"cable.yaml": true, "dir/billing.cable.yaml": true, ".cable.yaml": false, "cable.yml": false, "x.yaml": false} {
		if IsCollectionFile(name) != ok {
			t.Errorf("IsCollectionFile(%q) = %v", name, !ok)
		}
	}
}
