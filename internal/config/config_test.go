package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadRelayYAML(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "relay.yaml")
	content := "listen: \":443\"\n" +
		"host_key: /etc/nworm/host.key\n" +
		"keepalive: 30s\n" +
		"agents:\n" +
		"  home:\n" +
		"    keys: [\"ssh-ed25519 AAAA home\"]\n" +
		"admins:\n" +
		"  maciej:\n" +
		"    keys: [\"ssh-ed25519 BBBB maciej\"]\n" +
		"    allow: [home]\n"
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	var c Relay
	if err := Load(p, &c); err != nil {
		t.Fatal(err)
	}
	if c.Listen != ":443" {
		t.Fatalf("listen = %q", c.Listen)
	}
	if c.Keepalive.D() != 30*time.Second {
		t.Fatalf("keepalive = %v", c.Keepalive.D())
	}
	if got := c.Agents["home"].Keys[0]; got != "ssh-ed25519 AAAA home" {
		t.Fatalf("agent key = %q", got)
	}
	if len(c.Admins["maciej"].Allow) != 1 || c.Admins["maciej"].Allow[0] != "home" {
		t.Fatalf("allow = %v", c.Admins["maciej"].Allow)
	}
}

func TestLoadAgentYAML(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "agent.yaml")
	content := "name: home\n" +
		"identity: /etc/nworm/agent.key\n" +
		"relays: [\"relay1:443\", \"relay2:443\"]\n" +
		"remote: \"127.0.0.1:22\"\n" +
		"local: \"127.0.0.1:22\"\n" +
		"keepalive: 5s\n"
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	var c Agent
	if err := Load(p, &c); err != nil {
		t.Fatal(err)
	}
	if c.Name != "home" || len(c.Relays) != 2 || c.Remote != "127.0.0.1:22" {
		t.Fatalf("unexpected: %+v", c)
	}
}
