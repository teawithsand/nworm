package relay

import (
	"net"
	"path/filepath"
	"strings"
	"testing"

	"github.com/teawithsand/nworm/internal/config"
)

type textAddr string

func (a textAddr) Network() string { return "tcp" }
func (a textAddr) String() string  { return string(a) }

func TestOriginOf(t *testing.T) {
	cases := []struct {
		name     string
		addr     net.Addr
		wantHost string
		wantPort uint32
	}{
		{"ipv4 host and port", &net.TCPAddr{IP: net.ParseIP("10.1.2.3"), Port: 4242}, "10.1.2.3", 4242},
		{"ipv6 host and port", &net.TCPAddr{IP: net.ParseIP("::1"), Port: 7}, "::1", 7},
		{"zero port becomes one", &net.TCPAddr{IP: net.ParseIP("10.0.0.1"), Port: 0}, "10.0.0.1", 1},
		{"hostname is not an ip", textAddr("example.com:1234"), "127.0.0.1", 1234},
		{"missing port", textAddr("example.com"), "127.0.0.1", 1},
		{"port out of range", textAddr("10.0.0.1:99999"), "10.0.0.1", 1},
		{"negative port", textAddr("10.0.0.1:-5"), "10.0.0.1", 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			host, port := originOf(c.addr)
			if host != c.wantHost || port != c.wantPort {
				t.Fatalf("originOf(%v) = (%q, %d), want (%q, %d)", c.addr, host, port, c.wantHost, c.wantPort)
			}
		})
	}
}

func TestRunValidation(t *testing.T) {
	dir := t.TempDir()
	hostKey := filepath.Join(dir, "host.key")
	cases := []struct {
		name    string
		cfg     *config.Relay
		wantErr string
	}{
		{
			name:    "missing host key",
			cfg:     &config.Relay{Listen: "127.0.0.1:0"},
			wantErr: "host_key",
		},
		{
			name: "malformed agent key",
			cfg: &config.Relay{
				Listen:  "127.0.0.1:0",
				HostKey: hostKey,
				Agents:  map[string]config.AgentACL{"home": {Keys: []string{"not-a-key"}}},
			},
			wantErr: "agent",
		},
		{
			name: "malformed admin key",
			cfg: &config.Relay{
				Listen:  "127.0.0.1:0",
				HostKey: hostKey,
				Admins:  map[string]config.AdminACL{"admin": {Keys: []string{"not-a-key"}}},
			},
			wantErr: "admin",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := Run(c.cfg)
			if err == nil {
				t.Fatal("expected Run to fail during validation")
			}
			if !strings.Contains(err.Error(), c.wantErr) {
				t.Fatalf("error = %q, want substring %q", err, c.wantErr)
			}
		})
	}
}
