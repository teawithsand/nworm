package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

const version = "0.1.0"

// errUsage marks usage errors (bad flags/args/command), which exit with code 2.
var errUsage = errors.New("usage error")

// NewRoot builds the full command tree.
func NewRoot() *cobra.Command {
	root := &cobra.Command{
		Use:           "nworm",
		Short:         "Single-port SSH rendezvous for hosts behind NAT",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return fmt.Errorf("%w: %v", errUsage, err)
	})
	root.CompletionOptions.DisableDefaultCmd = true
	root.AddCommand(newRelayCmd(), newAgentCmd(), newConnectCmd(), newKeygenCmd(), newVersionCmd())
	return root
}

// Execute runs the CLI with os.Args and returns the process exit code.
func Execute() int {
	return Run(NewRoot(), os.Args[1:])
}

// Run executes a command tree with explicit args (used by tests).
func Run(root *cobra.Command, args []string) int {
	root.SetArgs(normalizeArgs(root, args))
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "nworm:", err)
		if errors.Is(err, errUsage) || strings.Contains(err.Error(), "unknown command") {
			return 2
		}
		return 1
	}
	return 0
}

func noArgs(_ *cobra.Command, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("%w: unexpected arguments %v", errUsage, args)
	}
	return nil
}

func exactArgs(n int) cobra.PositionalArgs {
	return func(_ *cobra.Command, args []string) error {
		if len(args) != n {
			return fmt.Errorf("%w: expected %d argument(s), got %d", errUsage, n, len(args))
		}
		return nil
	}
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version",
		Args:  noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), "nworm "+version)
			return nil
		},
	}
}

func flagString(cmd *cobra.Command, name string) string {
	v, _ := cmd.Flags().GetString(name)
	return v
}
