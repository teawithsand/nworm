package cli

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/teawithsand/nworm/internal/app"
	"github.com/teawithsand/nworm/internal/config"
)

func newConnectCmd() *cobra.Command {
	var listen string
	var port uint32
	cmd := &cobra.Command{
		Use:   "connect <name>",
		Short: "Connect to an agent through the relay",
		Args:  exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var cfg config.Connect
			if err := config.Resolve(cmd, &cfg, config.Options{
				Command:    "connect",
				ConfigFlag: flagString(cmd, "config"),
				Optional:   true,
				Aliases:    map[string]string{"admin_user": "user"},
			}); err != nil {
				return err
			}
			return app.RunConnect(&cfg, args[0], port, listen)
		},
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	def := filepath.Join(home, ".config", "nworm", "connect.yaml")
	cmd.Flags().String("config", def, "path to the connect config file (optional)")
	cmd.Flags().StringVar(&listen, "listen", "", "listen on a local address instead of proxying stdio")
	cmd.Flags().Uint32Var(&port, "port", 0, "target port on the agent (default from config)")
	cmd.Flags().String("identity", "", "override identity file")
	cmd.Flags().String("user", "", "override admin user")
	return cmd
}
