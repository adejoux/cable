package collection

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"
)

// DefaultFile is the collection file name looked up by default.
const DefaultFile = "cable.yaml"

// Suffix marks other collection files: billing.cable.yaml.
const Suffix = ".cable.yaml"

// ErrNotFound is returned by Find when no collection file is found.
var ErrNotFound = errors.New("no cable.yaml found in this directory or its parents")

// Load reads and decodes the collection at path, then indexes its recipes.
// It does not validate; call Validate for that.
func Load(path string) (*Collection, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	c, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	c.Path = path
	return c, nil
}

// Parse decodes a collection from YAML. Unknown keys are errors, with the
// closest known key suggested for recipes, folders, secrets and auth.
func Parse(data []byte) (*Collection, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	c := &Collection{}
	if err := dec.Decode(c); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, errors.New("the file is empty")
		}
		return nil, cleanYAMLError(err)
	}
	c.index()
	return c, nil
}

func cleanYAMLError(err error) error {
	msg := strings.TrimPrefix(err.Error(), "yaml: ")
	msg = strings.ReplaceAll(msg, "unmarshal errors:\n  ", "")
	if strings.Contains(msg, "did not find expected key") || strings.Contains(msg, "did not find expected node content") {
		msg += " (a value that starts with {{ must be quoted, e.g. url: \"{{ host }}/v1\")"
	}
	return errors.New(msg)
}

// index sets the path of every recipe.
func (c *Collection) index() {
	var walk func(items []Item, prefix string)
	walk = func(items []Item, prefix string) {
		for _, it := range items {
			switch {
			case it.Folder != nil:
				walk(it.Folder.Requests, prefix+it.Folder.Name+".")
			case it.Recipe != nil:
				it.Recipe.Path = prefix + it.Recipe.ID
			}
		}
	}
	walk(c.Requests, "")
}

// Recipes returns every recipe in collection order.
func (c *Collection) Recipes() []*Recipe {
	var out []*Recipe
	var walk func(items []Item)
	walk = func(items []Item) {
		for _, it := range items {
			if it.Folder != nil {
				walk(it.Folder.Requests)
			} else if it.Recipe != nil {
				out = append(out, it.Recipe)
			}
		}
	}
	walk(c.Requests)
	return out
}

// Recipe returns the recipe with the given path, such as "auth.login".
func (c *Collection) Recipe(path string) (*Recipe, bool) {
	for _, r := range c.Recipes() {
		if r.Path == path {
			return r, true
		}
	}
	return nil, false
}

// Find returns the nearest cable.yaml in dir or its parents.
func Find(dir string) (string, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for {
		path := filepath.Join(dir, DefaultFile)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, nil
		} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", ErrNotFound
		}
		dir = parent
	}
}

// IsCollectionFile reports whether name is cable.yaml or ends with
// .cable.yaml.
func IsCollectionFile(name string) bool {
	base := filepath.Base(name)
	return base == DefaultFile || (strings.HasSuffix(base, Suffix) && len(base) > len(Suffix))
}
