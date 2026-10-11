package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

func TestResolveEnvOverridesFileAndFlagOverridesEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "relay.yaml")
	if err := os.WriteFile(path, []byte("listen: \":443\"\nhost_key: /from/file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("NWORM_RELAY_LISTEN", ":9999")

	cmd := &cobra.Command{}
	cmd.Flags().String("listen", "", "")
	cmd.Flags().String("host-key", "", "")

	var fromEnv Relay
	if err := Resolve(cmd, &fromEnv, Options{Command: "relay", ConfigFlag: path}); err != nil {
		t.Fatal(err)
	}
	if fromEnv.Listen != ":9999" {
		t.Fatalf("env did not override file: listen = %q", fromEnv.Listen)
	}
	if fromEnv.HostKey != "/from/file" {
		t.Fatalf("file value lost: host_key = %q", fromEnv.HostKey)
	}

	if err := cmd.Flags().Set("listen", ":7777"); err != nil {
		t.Fatal(err)
	}
	var fromFlag Relay
	if err := Resolve(cmd, &fromFlag, Options{Command: "relay", ConfigFlag: path}); err != nil {
		t.Fatal(err)
	}
	if fromFlag.Listen != ":7777" {
		t.Fatalf("flag did not override env: listen = %q", fromFlag.Listen)
	}
}

func TestResolveEnvCSVList(t *testing.T) {
	t.Setenv("NWORM_AGENT_RELAYS", "a:443,b:443")
	var cfg Agent
	if err := Resolve(&cobra.Command{}, &cfg, Options{Command: "agent"}); err != nil {
		t.Fatal(err)
	}
	if len(cfg.Relays) != 2 || cfg.Relays[0] != "a:443" || cfg.Relays[1] != "b:443" {
		t.Fatalf("relays = %v", cfg.Relays)
	}
}

func TestResolveEnvJSONList(t *testing.T) {
	t.Setenv("NWORM_AGENT_RELAYS", `["a:443","b:443"]`)
	var cfg Agent
	if err := Resolve(&cobra.Command{}, &cfg, Options{Command: "agent"}); err != nil {
		t.Fatal(err)
	}
	if len(cfg.Relays) != 2 {
		t.Fatalf("relays = %v", cfg.Relays)
	}
}

func TestResolveEnvJSONMap(t *testing.T) {
	t.Setenv("NWORM_CONNECT_DEST", `{"home":22,"home-rdp":3389}`)
	var cfg Connect
	if err := Resolve(&cobra.Command{}, &cfg, Options{Command: "connect"}); err != nil {
		t.Fatal(err)
	}
	if cfg.Dest["home"] != 22 || cfg.Dest["home-rdp"] != 3389 {
		t.Fatalf("dest = %v", cfg.Dest)
	}
}

func TestResolveEnvJSONNested(t *testing.T) {
	t.Setenv("NWORM_RELAY_AGENTS", `{"home":{"keys":["ssh-ed25519 AAAA home"]}}`)
	var cfg Relay
	if err := Resolve(&cobra.Command{}, &cfg, Options{Command: "relay"}); err != nil {
		t.Fatal(err)
	}
	if got := cfg.Agents["home"].Keys; len(got) != 1 || got[0] != "ssh-ed25519 AAAA home" {
		t.Fatalf("agents = %+v", cfg.Agents)
	}
}

func TestResolveEnvEmptyClearsField(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "relay.yaml")
	if err := os.WriteFile(path, []byte("listen: \":443\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("NWORM_RELAY_LISTEN", "")
	var cfg Relay
	if err := Resolve(&cobra.Command{}, &cfg, Options{Command: "relay", ConfigFlag: path}); err != nil {
		t.Fatal(err)
	}
	if cfg.Listen != "" {
		t.Fatalf("empty env should clear the field, got %q", cfg.Listen)
	}
}
