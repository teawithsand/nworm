package cli

import (
	"github.com/spf13/cobra"

	"github.com/teawithsand/nworm/internal/app"
	"github.com/teawithsand/nworm/internal/config"
)

func newRelayCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "relay",
		Short: "Run the public relay (VPS)",
		Args:  noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			var cfg config.Relay
			if err := config.Resolve(cmd, &cfg, config.Options{
				Command:    "relay",
				ConfigFlag: flagString(cmd, "config"),
			}); err != nil {
				return err
			}
			return app.RunRelay(&cfg)
		},
	}
	cmd.Flags().String("config", "/etc/nworm/relay.yaml", "path to the relay config file")
	cmd.Flags().String("listen", "", "address to listen on (overrides config)")
	cmd.Flags().String("host-key", "", "path to the relay host key (overrides config)")
	cmd.Flags().Duration("keepalive", 0, "keepalive interval (overrides config)")
	return cmd
}
