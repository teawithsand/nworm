package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/teawithsand/nworm/internal/app"
)

func newKeygenCmd() *cobra.Command {
	var out string
	var comment string
	cmd := &cobra.Command{
		Use:   "keygen",
		Short: "Generate an ed25519 identity key",
		Args:  noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := app.RunKeygen(out, comment)
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), s)
			return nil
		},
	}
	cmd.Flags().StringVarP(&out, "output", "f", "", "output private key path")
	cmd.Flags().StringVar(&comment, "comment", "nworm", "key comment")
	return cmd
}
