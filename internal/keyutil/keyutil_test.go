package keyutil

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func newSigner(t *testing.T) ssh.Signer {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	return signer
}

func TestExpand(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no home directory: %v", err)
	}
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"tilde path expands", "~/keys/id", filepath.Join(home, "keys/id")},
		{"absolute path unchanged", "/etc/nworm/id", "/etc/nworm/id"},
		{"relative path unchanged", "keys/id", "keys/id"},
		{"bare tilde unchanged", "~", "~"},
		{"other user tilde unchanged", "~bob/id", "~bob/id"},
		{"empty unchanged", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Expand(c.in); got != c.want {
				t.Fatalf("Expand(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestGenerateIdentity(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "id_ed25519")
	pub, err := GenerateIdentity(path, "admin@test")
	if err != nil {
		t.Fatal(err)
	}
	if pub == nil {
		t.Fatal("nil public key")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("private key mode = %v, want 0600", info.Mode().Perm())
	}
	signer, err := LoadSigner(path)
	if err != nil {
		t.Fatalf("generated key does not load: %v", err)
	}
	if !bytes.Equal(signer.PublicKey().Marshal(), pub.Marshal()) {
		t.Fatal("public key does not match the generated private key")
	}
	dirInfo, err := os.Stat(filepath.Join(dir, "nested"))
	if err != nil {
		t.Fatal(err)
	}
	if dirInfo.Mode().Perm() != 0o700 {
		t.Fatalf("parent dir mode = %v, want 0700", dirInfo.Mode().Perm())
	}
}

func TestGenerateIdentityRequiresPath(t *testing.T) {
	if _, err := GenerateIdentity("", "x"); err == nil {
		t.Fatal("expected an error for an empty output path")
	}
}

func TestLoadSignerErrors(t *testing.T) {
	dir := t.TempDir()
	if _, err := LoadSigner(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("expected an error for a missing key file")
	}
	bad := filepath.Join(dir, "bad.key")
	if err := os.WriteFile(bad, []byte("this is not a private key"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadSigner(bad); err == nil {
		t.Fatal("expected an error for a malformed key file")
	}
}

func TestLoadOrCreateHostKeyIsStable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "host.key")
	first, err := LoadOrCreateHostKey(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("host key mode = %v, want 0600", info.Mode().Perm())
	}
	pubBytes, err := os.ReadFile(path + ".pub")
	if err != nil {
		t.Fatalf("public host key not written: %v", err)
	}
	parsed, _, _, _, err := ssh.ParseAuthorizedKey(pubBytes)
	if err != nil {
		t.Fatalf("public host key is not a valid authorized key: %v", err)
	}
	if !bytes.Equal(parsed.Marshal(), first.PublicKey().Marshal()) {
		t.Fatal("written public key does not match the private key")
	}
	second, err := LoadOrCreateHostKey(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(second.PublicKey().Marshal(), first.PublicKey().Marshal()) {
		t.Fatal("host key changed on reload")
	}
}

func TestLoadOrCreateHostKeyErrors(t *testing.T) {
	if _, err := LoadOrCreateHostKey(""); err == nil {
		t.Fatal("expected an error for an empty host key path")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "host.key")
	if err := os.WriteFile(path, []byte("not a key"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadOrCreateHostKey(path); err == nil {
		t.Fatal("expected an error for a malformed host key")
	}
}

func TestHostKeyCallbackKnownHosts(t *testing.T) {
	dir := t.TempDir()
	key := newSigner(t).PublicKey()
	knownHosts := filepath.Join(dir, "known_hosts")
	line := knownhosts.Line([]string{"[127.0.0.1]:2222"}, key) + "\n"
	if err := os.WriteFile(knownHosts, []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}
	cb, err := HostKeyCallback(knownHosts, false)
	if err != nil {
		t.Fatal(err)
	}
	addr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 2222}
	if err := cb("127.0.0.1:2222", addr, key); err != nil {
		t.Fatalf("pinned host key rejected: %v", err)
	}
	wrong := newSigner(t).PublicKey()
	if err := cb("127.0.0.1:2222", addr, wrong); err == nil {
		t.Fatal("a different host key was accepted despite pinning")
	}
}

func TestHostKeyCallbackFallbacks(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "absent_known_hosts")
	if _, err := HostKeyCallback(missing, false); err == nil {
		t.Fatal("expected an error when known_hosts is missing and insecure is false")
	}
	if _, err := HostKeyCallback("", false); err == nil {
		t.Fatal("expected an error when no known_hosts and insecure is false")
	}
	cb, err := HostKeyCallback(missing, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := cb("anywhere:22", nil, newSigner(t).PublicKey()); err != nil {
		t.Fatalf("insecure callback rejected a key: %v", err)
	}
	cb, err = HostKeyCallback("", true)
	if err != nil {
		t.Fatal(err)
	}
	if err := cb("anywhere:22", nil, newSigner(t).PublicKey()); err != nil {
		t.Fatalf("insecure callback rejected a key: %v", err)
	}
}

func TestParseAuthorizedKeys(t *testing.T) {
	line := string(ssh.MarshalAuthorizedKey(newSigner(t).PublicKey()))
	keys, err := ParseAuthorizedKeys([]string{line})
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 {
		t.Fatalf("parsed %d keys, want 1", len(keys))
	}
	if _, err := ParseAuthorizedKeys([]string{"ssh-ed25519 not-base64 user@host"}); err == nil {
		t.Fatal("expected an error for a malformed authorized key")
	}
	if keys, err := ParseAuthorizedKeys(nil); err != nil || len(keys) != 0 {
		t.Fatalf("ParseAuthorizedKeys(nil) = %v, %v; want empty, nil", keys, err)
	}
}

func TestKeyAllowed(t *testing.T) {
	allowed := newSigner(t).PublicKey()
	other := newSigner(t).PublicKey()
	if !KeyAllowed(allowed, []ssh.PublicKey{allowed}) {
		t.Fatal("matching key was rejected")
	}
	if KeyAllowed(other, []ssh.PublicKey{allowed}) {
		t.Fatal("non-matching key was accepted")
	}
	if KeyAllowed(allowed, nil) {
		t.Fatal("empty allow list accepted a key")
	}
}
