package cli

import (
	"github.com/spf13/cobra"

	"github.com/teawithsand/nworm/internal/app"
	"github.com/teawithsand/nworm/internal/config"
)

func newAgentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Run the agent on a host behind NAT",
		Args:  noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			var cfg config.Agent
			if err := config.Resolve(cmd, &cfg, config.Options{
				Command:    "agent",
				ConfigFlag: flagString(cmd, "config"),
			}); err != nil {
				return err
			}
			return app.RunAgent(&cfg)
		},
	}
	cmd.Flags().String("config", "/etc/nworm/agent.yaml", "path to the agent config file")
	cmd.Flags().String("name", "", "agent name (overrides config)")
	cmd.Flags().String("identity", "", "identity key path (overrides config)")
	cmd.Flags().StringArray("relays", nil, "relay addresses, repeatable (overrides config)")
	cmd.Flags().String("remote", "", "address to register on the relay (overrides config)")
	cmd.Flags().String("local", "", "local address to dial (overrides config)")
	cmd.Flags().String("known-hosts", "", "known_hosts file (overrides config)")
	cmd.Flags().Bool("insecure", false, "skip relay host key verification (testing only)")
	cmd.Flags().Duration("keepalive", 0, "keepalive interval (overrides config)")
	return cmd
}
