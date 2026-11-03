// Package collection loads and validates cable collections: the recipes,
// folders, profiles, secrets and prompts of a cable.yaml file.
package collection

import (
	"fmt"
	"time"

	"go.yaml.in/yaml/v3"
)

// Version is the collection format version this package reads.
const Version = 1

// Collection is a parsed cable.yaml file.
type Collection struct {
	Version  int                `yaml:"version"`
	Name     string             `yaml:"name"`
	Vars     map[string]string  `yaml:"vars"`
	Defaults Defaults           `yaml:"defaults"`
	Profiles map[string]Profile `yaml:"profiles"`
	Secrets  map[string]Secret  `yaml:"secrets"`
	Prompts  map[string]Prompt  `yaml:"prompts"`
	OpenAPI  string             `yaml:"openapi"`
	History  yaml.Node          `yaml:"history"`
	Requests []Item             `yaml:"requests"`

	// Path is the file the collection was loaded from.
	Path string `yaml:"-"`
}

// Defaults apply to every recipe unless a folder or the recipe overrides
// them.
type Defaults struct {
	Headers map[string]string `yaml:"headers"`
	Timeout Duration          `yaml:"timeout"`
	Auth    *Auth             `yaml:"auth"`
}

// Profile is a named environment.
type Profile struct {
	Vars map[string]string `yaml:"vars"`
	// Confirm asks before every send with this profile.
	Confirm bool `yaml:"confirm"`
}

// Remember says how long a value typed at a prompt is kept.
type Remember string

const (
	// RememberSend keeps the value for one send only.
	RememberSend Remember = "send"
	// RememberSession keeps the value in memory until cable exits.
	RememberSession Remember = "session"
)

// Secret is a declared secret. Its value never lives in the collection: From
// lists the providers tried in order.
type Secret struct {
	Description string   `yaml:"description"`
	From        []string `yaml:"from"`
	Remember    Remember `yaml:"remember"`

	// Value is only read to reject it with a clear message.
	Value *string `yaml:"value"`
	Line  int     `yaml:"-"`
}

// Prompt is a value asked at send time and shown in clear.
type Prompt struct {
	Description string   `yaml:"description"`
	Default     string   `yaml:"default"`
	Choices     []string `yaml:"choices"`
}

// Item is one entry of a requests list: a folder or a recipe.
type Item struct {
	Folder *Folder
	Recipe *Recipe
}

// Folder groups recipes and can set headers and auth for them.
type Folder struct {
	Name     string            `yaml:"folder"`
	Headers  map[string]string `yaml:"headers"`
	Auth     *Auth             `yaml:"auth"`
	Requests []Item            `yaml:"requests"`
	Line     int               `yaml:"-"`
}

// Recipe is a request template.
type Recipe struct {
	ID         string            `yaml:"id"`
	Name       string            `yaml:"name"`
	Method     string            `yaml:"method"`
	URL        string            `yaml:"url"`
	Query      map[string]string `yaml:"query"`
	Headers    map[string]string `yaml:"headers"`
	Body       *Body             `yaml:"body"`
	Auth       *Auth             `yaml:"auth"`
	Timeout    Duration          `yaml:"timeout"`
	Trigger    string            `yaml:"trigger"`
	RedactJSON []string          `yaml:"redact_json"`

	// Parsed by later milestones; kept as YAML for now.
	Capture  yaml.Node `yaml:"capture"`
	Expect   yaml.Node `yaml:"expect"`
	Paginate yaml.Node `yaml:"paginate"`
	Foreach  yaml.Node `yaml:"foreach"`

	// Path is the recipe path, folder names and id joined by dots.
	Path string `yaml:"-"`
	Line int    `yaml:"-"`
}

// Body is a request body; exactly one kind is set.
type Body struct {
	JSON        any               `yaml:"json"`
	Form        map[string]string `yaml:"form"`
	Multipart   map[string]string `yaml:"multipart"`
	Text        *string           `yaml:"text"`
	File        string            `yaml:"file"`
	ContentType string            `yaml:"content_type"`
}

// Kind names the body kind, or "" when none is set.
func (b *Body) Kind() string {
	if b == nil {
		return ""
	}
	kinds := b.kinds()
	if len(kinds) != 1 {
		return ""
	}
	return kinds[0]
}

func (b *Body) kinds() []string {
	var out []string
	if b.JSON != nil {
		out = append(out, "json")
	}
	if b.Form != nil {
		out = append(out, "form")
	}
	if b.Multipart != nil {
		out = append(out, "multipart")
	}
	if b.Text != nil {
		out = append(out, "text")
	}
	if b.File != "" {
		out = append(out, "file")
	}
	return out
}

// Auth is the auth of a recipe, folder or the defaults: "none", or a mapping
// with exactly one scheme.
type Auth struct {
	None     bool
	Basic    *BasicAuth    `yaml:"basic"`
	Bearer   *string       `yaml:"bearer"`
	APIKey   *APIKeyAuth   `yaml:"api_key"`
	OIDC     *OIDCAuth     `yaml:"oidc"`
	IBMCloud *IBMCloudAuth `yaml:"ibmcloud"`
	Line     int           `yaml:"-"`
}

// BasicAuth is HTTP basic auth.
type BasicAuth struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

// APIKeyAuth sends a key in a header or a query parameter.
type APIKeyAuth struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
	In    string `yaml:"in"`
}

// OIDCAuth gets a token from an OIDC provider through authkit.
type OIDCAuth struct {
	Issuer       string   `yaml:"issuer"`
	ClientID     string   `yaml:"client_id"`
	ClientSecret string   `yaml:"client_secret"`
	Grant        string   `yaml:"grant"`
	Scopes       []string `yaml:"scopes"`
	Audience     string   `yaml:"audience"`
}

// IBMCloudAuth exchanges an IBM Cloud API key for a token through authkit.
type IBMCloudAuth struct {
	APIKey      string `yaml:"api_key"`
	IAMEndpoint string `yaml:"iam_endpoint"`
}

// Scheme names the auth scheme: "none", "basic", "bearer", "api_key",
// "oidc" or "ibmcloud"; "" when none or several are set.
func (a *Auth) Scheme() string {
	if a == nil {
		return ""
	}
	if s := a.schemes(); len(s) == 1 {
		return s[0]
	}
	return ""
}

func (a *Auth) schemes() []string {
	var out []string
	if a.None {
		out = append(out, "none")
	}
	if a.Basic != nil {
		out = append(out, "basic")
	}
	if a.Bearer != nil {
		out = append(out, "bearer")
	}
	if a.APIKey != nil {
		out = append(out, "api_key")
	}
	if a.OIDC != nil {
		out = append(out, "oidc")
	}
	if a.IBMCloud != nil {
		out = append(out, "ibmcloud")
	}
	return out
}

// Duration is a time.Duration written as "30s" or "2m" in YAML.
type Duration time.Duration

// UnmarshalYAML parses a Go duration string.
func (d *Duration) UnmarshalYAML(n *yaml.Node) error {
	var s string
	if err := n.Decode(&s); err != nil {
		return err
	}
	if s == "" {
		*d = 0
		return nil
	}
	v, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("line %d: invalid duration %q: use a value such as 30s or 2m", n.Line, s)
	}
	*d = Duration(v)
	return nil
}

// Std returns the duration as a time.Duration.
func (d Duration) Std() time.Duration { return time.Duration(d) }
