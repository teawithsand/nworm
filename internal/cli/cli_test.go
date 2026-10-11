package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeArgs(t *testing.T) {
	root := NewRoot()
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{"single-dash long", []string{"relay", "-config", "x.yaml"}, []string{"relay", "--config", "x.yaml"}},
		{"single-dash with value", []string{"relay", "-config=x.yaml"}, []string{"relay", "--config=x.yaml"}},
		{"double dash stays", []string{"relay", "--config", "x"}, []string{"relay", "--config", "x"}},
		{"shorthand stays", []string{"keygen", "-f", "k"}, []string{"keygen", "-f", "k"}},
		{"positional stays", []string{"connect", "home"}, []string{"connect", "home"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := normalizeArgs(root, c.in)
			if strings.Join(got, " ") != strings.Join(c.want, " ") {
				t.Fatalf("normalizeArgs(%v) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}

func TestVersionCommand(t *testing.T) {
	root := NewRoot()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"version"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "nworm") {
		t.Fatalf("version output = %q", buf.String())
	}
}

func TestKeygenCommand(t *testing.T) {
	dir := t.TempDir()
	key := filepath.Join(dir, "id_ed25519")
	root := NewRoot()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"keygen", "-f", key})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(key); err != nil {
		t.Fatalf("private key not written: %v", err)
	}
	if !strings.HasPrefix(buf.String(), "ssh-ed25519 ") {
		t.Fatalf("public key output = %q", buf.String())
	}
}

func TestUnknownCommandExitCode(t *testing.T) {
	if code := Run(NewRoot(), []string{"definitely-not-a-command"}); code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

func TestUnknownFlagExitCode(t *testing.T) {
	if code := Run(NewRoot(), []string{"relay", "--not-a-flag"}); code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}
