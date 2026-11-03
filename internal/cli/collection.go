package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/adejoux/cable/internal/collection"
)

// errInvalid is returned when a collection has validation problems; the
// problems are printed before.
var errInvalid = errors.New("the collection is not valid")

// collectionPath returns the collection given with -c, else the nearest
// cable.yaml from the working directory.
func (o *Options) collectionPath() (string, error) {
	if o.Collection != "" {
		return o.Collection, nil
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	path, err := collection.Find(wd)
	if errors.Is(err, collection.ErrNotFound) {
		return "", fmt.Errorf("%w; use -c to name a collection file", err)
	}
	return path, err
}

// load reads the collection named by the options.
func (o *Options) load() (*collection.Collection, error) {
	path, err := o.collectionPath()
	if err != nil {
		return nil, err
	}
	return collection.Load(path)
}

func newValidateCommand(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Check the collection for errors",
		Long:  "Check the collection file: YAML syntax, unknown keys, names, recipes, auth, secrets and prompts.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := opts.load()
			if err != nil {
				return err
			}
			return printValidation(cmd.OutOrStdout(), c)
		},
	}
}

func printValidation(w io.Writer, c *collection.Collection) error {
	problems := c.Validate()
	if len(problems) == 0 {
		_, err := fmt.Fprintf(w, "%s: ok, %d recipes\n", c.Path, len(c.Recipes()))
		return err
	}
	for _, p := range problems {
		line := ""
		if p.Line > 0 {
			line = fmt.Sprintf(":%d", p.Line)
		}
		if _, err := fmt.Fprintf(w, "%s%s: %s\n", c.Path, line, p.Message); err != nil {
			return err
		}
	}
	return fmt.Errorf("%w: %d problems", errInvalid, len(problems))
}

func newListCommand(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List the recipes of the collection",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := opts.load()
			if err != nil {
				return err
			}
			return printRecipes(cmd.OutOrStdout(), c)
		},
	}
}

func printRecipes(w io.Writer, c *collection.Collection) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "RECIPE\tMETHOD\tURL"); err != nil {
		return err
	}
	for _, r := range c.Recipes() {
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\n", r.Path, r.Method, r.URL); err != nil {
			return err
		}
	}
	return tw.Flush()
}
