package app

import (
	"golang.org/x/crypto/ssh"

	"github.com/teawithsand/nworm/internal/agent"
	"github.com/teawithsand/nworm/internal/config"
	"github.com/teawithsand/nworm/internal/connect"
	"github.com/teawithsand/nworm/internal/keyutil"
	"github.com/teawithsand/nworm/internal/relay"
)

// RunRelay starts the public relay.
func RunRelay(cfg *config.Relay) error { return relay.Run(cfg) }

// RunAgent starts the agent on a host behind NAT.
func RunAgent(cfg *config.Agent) error { return agent.Run(cfg) }

// RunConnect bridges an admin client to an agent through the relay.
func RunConnect(cfg *config.Connect, name string, port uint32, listen string) error {
	return connect.Run(cfg, name, port, listen)
}

// RunKeygen generates an ed25519 identity and returns the authorized key line.
func RunKeygen(out, comment string) (string, error) {
	pub, err := keyutil.GenerateIdentity(out, comment)
	if err != nil {
		return "", err
	}
	return string(ssh.MarshalAuthorizedKey(pub)), nil
}
