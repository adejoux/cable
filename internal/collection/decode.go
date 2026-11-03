package collection

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

// UnmarshalYAML reads a folder when the mapping has a folder key, else a
// recipe.
func (it *Item) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind != yaml.MappingNode {
		return fmt.Errorf("line %d: a request entry must be a mapping with an id (recipe) or a folder", n.Line)
	}
	if hasKey(n, "folder") {
		f := &Folder{}
		if err := f.UnmarshalYAML(n); err != nil {
			return err
		}
		it.Folder = f
		return nil
	}
	r := &Recipe{}
	if err := r.UnmarshalYAML(n); err != nil {
		return err
	}
	it.Recipe = r
	return nil
}

// UnmarshalYAML decodes a folder and rejects unknown keys.
func (f *Folder) UnmarshalYAML(n *yaml.Node) error {
	type plain Folder
	if err := strictDecode(n, (*plain)(f), "folder"); err != nil {
		return err
	}
	f.Line = n.Line
	return nil
}

// UnmarshalYAML decodes a recipe and rejects unknown keys.
func (r *Recipe) UnmarshalYAML(n *yaml.Node) error {
	type plain Recipe
	if err := strictDecode(n, (*plain)(r), "recipe"); err != nil {
		return err
	}
	r.Line = n.Line
	return nil
}

// UnmarshalYAML decodes a secret declaration and rejects unknown keys.
func (s *Secret) UnmarshalYAML(n *yaml.Node) error {
	type plain Secret
	if err := strictDecode(n, (*plain)(s), "secret"); err != nil {
		return err
	}
	s.Line = n.Line
	return nil
}

// UnmarshalYAML reads "none" or a mapping with one auth scheme.
func (a *Auth) UnmarshalYAML(n *yaml.Node) error {
	a.Line = n.Line
	if n.Kind == yaml.ScalarNode {
		if n.Value != "none" {
			return fmt.Errorf("line %d: auth must be none or a mapping such as bearer: …", n.Line)
		}
		a.None = true
		return nil
	}
	type plain Auth
	return strictDecode(n, (*plain)(a), "auth")
}

// strictDecode decodes n into out and fails on keys that out does not
// declare, naming the closest known key. what names the object in errors.
func strictDecode(n *yaml.Node, out any, what string) error {
	if n.Kind != yaml.MappingNode {
		return fmt.Errorf("line %d: %s must be a mapping", n.Line, what)
	}
	known := yamlKeys(reflect.TypeOf(out).Elem())
	for i := 0; i+1 < len(n.Content); i += 2 {
		k := n.Content[i]
		if !known[k.Value] {
			msg := fmt.Sprintf("line %d: unknown %s key %q", k.Line, what, k.Value)
			if near := closest(k.Value, known); near != "" {
				msg += fmt.Sprintf(" (did you mean %q?)", near)
			}
			return fmt.Errorf("%s", msg)
		}
	}
	return n.Decode(out)
}

func yamlKeys(t reflect.Type) map[string]bool {
	keys := map[string]bool{}
	for i := 0; i < t.NumField(); i++ {
		tag := strings.Split(t.Field(i).Tag.Get("yaml"), ",")[0]
		if tag != "" && tag != "-" {
			keys[tag] = true
		}
	}
	return keys
}

func hasKey(n *yaml.Node, key string) bool {
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return true
		}
	}
	return false
}

// closest returns the known key nearest to s, if it is a plausible typo.
func closest(s string, known map[string]bool) string {
	names := make([]string, 0, len(known))
	for k := range known {
		names = append(names, k)
	}
	sort.Strings(names)
	best, bestDist := "", 3
	for _, k := range names {
		if d := distance(s, k); d < bestDist {
			best, bestDist = k, d
		}
	}
	return best
}

// distance is the Levenshtein distance between a and b.
func distance(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(rb)]
}
