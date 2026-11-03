// Package cli holds cable's Cobra commands.
package cli

import (
	"errors"
	"fmt"
	"io"
	"runtime/debug"

	"github.com/spf13/cobra"
)

// version is set at build time with
// -ldflags "-X github.com/adejoux/cable/internal/cli.version=v1.2.3".
var version = ""

// errNoTUI is returned by cable without a subcommand until the TUI exists.
var errNoTUI = errors.New("the TUI is not available yet; run cable --help for the commands")

// Options are the flags shared by every command.
type Options struct {
	// Collection is the collection file given with -c. Empty means the
	// nearest cable.yaml, then the last collection used.
	Collection string
	// Profile is the profile given with -p. Empty means the collection's
	// default profile.
	Profile string
}

// Execute runs the command line.
func Execute() error {
	return NewRootCommand().Execute()
}

// NewRootCommand returns the cable command tree.
func NewRootCommand() *cobra.Command {
	opts := &Options{}
	root := &cobra.Command{
		Use:           "cable",
		Short:         "Terminal HTTP client with collections as files",
		Long:          "cable sends HTTP requests defined as recipes in a cable.yaml collection, from a TUI or from scripts.",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return errNoTUI
		},
	}
	root.PersistentFlags().StringVarP(&opts.Collection, "collection", "c", "", "collection file (default: nearest cable.yaml)")
	root.PersistentFlags().StringVarP(&opts.Profile, "profile", "p", "", "profile to use")

	root.AddCommand(newVersionCommand(), newListCommand(opts), newValidateCommand(opts))
	return root
}

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the cable version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return printVersion(cmd.OutOrStdout())
		},
	}
}

func printVersion(w io.Writer) error {
	_, err := fmt.Fprintln(w, "cable", Version())
	return err
}

// Version returns the build version: the ldflags value, else the module
// version recorded by go install, else "dev".
func Version() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}
