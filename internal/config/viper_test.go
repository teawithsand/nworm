package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestResolveFileThenFlagPrecedence(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "relay.yaml", "listen: \":443\"\nhost_key: /from/file\n")

	cmd := &cobra.Command{}
	cmd.Flags().String("listen", "", "")
	cmd.Flags().String("host-key", "", "")

	var fromFile Relay
	if err := Resolve(cmd, &fromFile, Options{Command: "relay", ConfigFlag: path}); err != nil {
		t.Fatal(err)
	}
	if fromFile.Listen != ":443" || fromFile.HostKey != "/from/file" {
		t.Fatalf("file config = %+v", fromFile)
	}

	if err := cmd.Flags().Set("listen", ":8443"); err != nil {
		t.Fatal(err)
	}
	var withFlag Relay
	if err := Resolve(cmd, &withFlag, Options{Command: "relay", ConfigFlag: path}); err != nil {
		t.Fatal(err)
	}
	if withFlag.Listen != ":8443" {
		t.Fatalf("flag did not override file: listen = %q", withFlag.Listen)
	}
	if withFlag.HostKey != "/from/file" {
		t.Fatalf("file value lost: host_key = %q", withFlag.HostKey)
	}
}

func TestResolveOptionalMissingFile(t *testing.T) {
	cmd := &cobra.Command{}
	var cfg Connect
	if err := Resolve(cmd, &cfg, Options{Command: "connect", ConfigFlag: filepath.Join(t.TempDir(), "nope.yaml"), Optional: true}); err != nil {
		t.Fatalf("optional missing file should not error: %v", err)
	}
}

func TestResolveRequiredMissingFile(t *testing.T) {
	cmd := &cobra.Command{}
	var cfg Relay
	if err := Resolve(cmd, &cfg, Options{Command: "relay", ConfigFlag: filepath.Join(t.TempDir(), "nope.yaml")}); err == nil {
		t.Fatal("expected an error for a missing required config file")
	}
}

func TestResolveDuration(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "agent.yaml", "name: home\nkeepalive: 45s\n")
	cmd := &cobra.Command{}
	var cfg Agent
	if err := Resolve(cmd, &cfg, Options{Command: "agent", ConfigFlag: path}); err != nil {
		t.Fatal(err)
	}
	if cfg.Keepalive.D().Seconds() != 45 {
		t.Fatalf("keepalive = %v", cfg.Keepalive.D())
	}
}
