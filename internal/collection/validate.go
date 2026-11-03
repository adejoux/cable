package collection

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
)

// Problem is one validation finding, located by line when known.
type Problem struct {
	Line    int
	Message string
}

func (p Problem) String() string {
	if p.Line > 0 {
		return fmt.Sprintf("line %d: %s", p.Line, p.Message)
	}
	return p.Message
}

// ReservedRoots are the template roots. No variable, folder, recipe,
// secret, prompt or profile may use these names.
var ReservedRoots = []string{"secret", "prompt", "env", "response", "item", "loop", "file", "gen", "profile"}

var identRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*$`)

// IsIdent reports whether s is a valid identifier: a letter or underscore,
// then letters, digits, underscores or dashes.
func IsIdent(s string) bool { return identRe.MatchString(s) }

// IsReserved reports whether s is a reserved template root.
func IsReserved(s string) bool { return slices.Contains(ReservedRoots, s) }

var methodRe = regexp.MustCompile(`^[A-Z]+$`)

// Validate checks the structure of the collection and returns the problems
// found, sorted by line. Template expressions are checked separately.
func (c *Collection) Validate() []Problem {
	v := &validator{}
	if c.Version == 0 {
		v.add(0, "version is missing: add version: 1 at the top")
	} else if c.Version != Version {
		v.add(0, fmt.Sprintf("version %d is not supported: this cable reads version %d", c.Version, Version))
	}
	v.names("variable", c.Vars)
	for _, name := range sortedKeys(c.Profiles) {
		v.name("profile", name, 0)
		v.names("variable", c.Profiles[name].Vars)
	}
	for _, name := range sortedKeys(c.Secrets) {
		v.secret(name, c.Secrets[name])
	}
	for _, name := range sortedKeys(c.Prompts) {
		p := c.Prompts[name]
		v.name("prompt", name, 0)
		if p.Default != "" && len(p.Choices) > 0 && !slices.Contains(p.Choices, p.Default) {
			v.add(0, fmt.Sprintf("prompt %s: default %q is not one of its choices", name, p.Default))
		}
	}
	v.auth(c.Defaults.Auth, "defaults")
	if len(c.Requests) == 0 {
		v.add(0, "the collection has no requests")
	}
	v.items(c.Requests, "", map[string]int{})
	sort.SliceStable(v.problems, func(i, j int) bool { return v.problems[i].Line < v.problems[j].Line })
	return v.problems
}

type validator struct {
	problems []Problem
}

func (v *validator) add(line int, msg string) {
	v.problems = append(v.problems, Problem{Line: line, Message: msg})
}

func (v *validator) name(kind, name string, line int) bool {
	switch {
	case !IsIdent(name):
		v.add(line, fmt.Sprintf("%s name %q must start with a letter or _ and use only letters, digits, _ and -", kind, name))
	case IsReserved(name):
		v.add(line, fmt.Sprintf("%s name %q is reserved for templates (%s)", kind, name, strings.Join(ReservedRoots, ", ")))
	default:
		return true
	}
	return false
}

func (v *validator) names(kind string, m map[string]string) {
	for _, name := range sortedKeys(m) {
		v.name(kind, name, 0)
	}
}

func (v *validator) secret(name string, s Secret) {
	v.name("secret", name, s.Line)
	if s.Value != nil {
		v.add(s.Line, fmt.Sprintf("secret %s: values are not allowed in the collection; store it with cable secrets set %s", name, name))
	}
	if len(s.From) == 0 {
		v.add(s.Line, fmt.Sprintf("secret %s: from is empty; list its providers, e.g. from: [file, prompt]", name))
	}
	for _, p := range s.From {
		if msg := checkProvider(p); msg != "" {
			v.add(s.Line, fmt.Sprintf("secret %s: %s", name, msg))
		}
	}
	switch s.Remember {
	case "", RememberSend, RememberSession:
	default:
		v.add(s.Line, fmt.Sprintf("secret %s: remember must be send or session, not %q", name, s.Remember))
	}
}

// checkProvider returns a problem with a secret provider, or "".
func checkProvider(p string) string {
	switch {
	case p == "file", p == "keyring", p == "prompt":
		return ""
	case strings.HasPrefix(p, "env:"):
		if strings.TrimSpace(strings.TrimPrefix(p, "env:")) == "" {
			return "env: needs a variable name, e.g. env:API_KEY"
		}
		return ""
	case strings.HasPrefix(p, "cmd:"):
		if strings.TrimSpace(strings.TrimPrefix(p, "cmd:")) == "" {
			return "cmd: needs a command, e.g. cmd:pass show api/key"
		}
		return ""
	default:
		return fmt.Sprintf("unknown provider %q: use file, env:NAME, cmd:COMMAND, keyring or prompt", p)
	}
}

func (v *validator) items(items []Item, prefix string, seen map[string]int) {
	for _, it := range items {
		switch {
		case it.Folder != nil:
			f := it.Folder
			if v.name("folder", f.Name, f.Line) {
				path := prefix + f.Name
				if line, dup := seen["folder:"+path]; dup {
					v.add(f.Line, fmt.Sprintf("folder %s is already defined at line %d", path, line))
				}
				seen["folder:"+path] = f.Line
			}
			v.auth(f.Auth, "folder "+prefix+f.Name)
			if len(f.Requests) == 0 {
				v.add(f.Line, fmt.Sprintf("folder %s has no requests", prefix+f.Name))
			}
			v.items(f.Requests, prefix+f.Name+".", seen)
		case it.Recipe != nil:
			v.recipe(it.Recipe, seen)
		}
	}
}

func (v *validator) recipe(r *Recipe, seen map[string]int) {
	if r.ID == "" {
		v.add(r.Line, "recipe without id: add id: <name>")
	} else if v.name("recipe", r.ID, r.Line) {
		if line, dup := seen["recipe:"+r.Path]; dup {
			v.add(r.Line, fmt.Sprintf("recipe %s is already defined at line %d", r.Path, line))
		}
		seen["recipe:"+r.Path] = r.Line
	}
	label := "recipe " + r.Path
	switch {
	case r.Method == "":
		v.add(r.Line, label+": method is missing")
	case !methodRe.MatchString(r.Method):
		v.add(r.Line, fmt.Sprintf("%s: method %q must be upper case, e.g. GET or POST", label, r.Method))
	}
	if strings.TrimSpace(r.URL) == "" {
		v.add(r.Line, label+": url is missing")
	}
	if r.Body != nil {
		switch kinds := r.Body.kinds(); len(kinds) {
		case 0:
			v.add(r.Line, label+": body needs one of json, form, multipart, text or file")
		case 1:
		default:
			v.add(r.Line, fmt.Sprintf("%s: body has several kinds (%s); keep one", label, strings.Join(kinds, ", ")))
		}
	}
	v.auth(r.Auth, label)
	switch r.Trigger {
	case "", "auto":
	default:
		v.add(r.Line, fmt.Sprintf("%s: trigger must be auto, not %q", label, r.Trigger))
	}
}

var oidcGrants = []string{"client_credentials", "password", "token_exchange"}

func (v *validator) auth(a *Auth, where string) {
	if a == nil {
		return
	}
	schemes := a.schemes()
	switch len(schemes) {
	case 0:
		v.add(a.Line, where+": auth is empty; use none or one of basic, bearer, api_key, oidc, ibmcloud")
		return
	case 1:
	default:
		v.add(a.Line, fmt.Sprintf("%s: auth has several schemes (%s); keep one", where, strings.Join(schemes, ", ")))
		return
	}
	missing := func(field string) {
		v.add(a.Line, fmt.Sprintf("%s: %s auth needs %s", where, schemes[0], field))
	}
	switch {
	case a.Basic != nil && a.Basic.Username == "":
		missing("username")
	case a.Bearer != nil && strings.TrimSpace(*a.Bearer) == "":
		missing("a token")
	case a.APIKey != nil:
		if a.APIKey.Name == "" {
			missing("name")
		}
		if a.APIKey.Value == "" {
			missing("value")
		}
		if in := a.APIKey.In; in != "" && in != "header" && in != "query" {
			v.add(a.Line, fmt.Sprintf("%s: api_key in must be header or query, not %q", where, in))
		}
	case a.OIDC != nil:
		if a.OIDC.Issuer == "" {
			missing("issuer")
		}
		if a.OIDC.ClientID == "" {
			missing("client_id")
		}
		if g := a.OIDC.Grant; g != "" && !slices.Contains(oidcGrants, g) {
			v.add(a.Line, fmt.Sprintf("%s: oidc grant must be one of %s, not %q", where, strings.Join(oidcGrants, ", "), g))
		}
	case a.IBMCloud != nil && a.IBMCloud.APIKey == "":
		missing("api_key")
	}
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
