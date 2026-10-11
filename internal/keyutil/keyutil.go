package keyutil

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func Expand(p string) string {
	if strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, p[2:])
		}
	}
	return p
}

func LoadSigner(path string) (ssh.Signer, error) {
	b, err := os.ReadFile(Expand(path))
	if err != nil {
		return nil, err
	}
	return ssh.ParsePrivateKey(b)
}

func GenerateIdentity(path, comment string) (ssh.PublicKey, error) {
	if path == "" {
		return nil, fmt.Errorf("output path is required")
	}
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	block, err := ssh.MarshalPrivateKey(priv, comment)
	if err != nil {
		return nil, err
	}
	p := Expand(path)
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return nil, err
	}
	if err := os.WriteFile(p, pem.EncodeToMemory(block), 0o600); err != nil {
		return nil, err
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		return nil, err
	}
	return signer.PublicKey(), nil
}

func LoadOrCreateHostKey(path string) (ssh.Signer, error) {
	if path == "" {
		return nil, fmt.Errorf("host_key is required")
	}
	p := Expand(path)
	if b, err := os.ReadFile(p); err == nil {
		signer, err := ssh.ParsePrivateKey(b)
		if err != nil {
			return nil, err
		}
		writePublic(p, signer)
		return signer, nil
	}
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	block, err := ssh.MarshalPrivateKey(priv, "nworm host key")
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return nil, err
	}
	if err := os.WriteFile(p, pem.EncodeToMemory(block), 0o600); err != nil {
		return nil, err
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		return nil, err
	}
	writePublic(p, signer)
	return signer, nil
}

func writePublic(privPath string, signer ssh.Signer) {
	_ = os.WriteFile(privPath+".pub", ssh.MarshalAuthorizedKey(signer.PublicKey()), 0o644)
}

func HostKeyCallback(knownHosts string, insecure bool) (ssh.HostKeyCallback, error) {
	if knownHosts != "" {
		p := Expand(knownHosts)
		if _, err := os.Stat(p); err == nil {
			return knownhosts.New(p)
		}
	}
	if insecure {
		return ssh.InsecureIgnoreHostKey(), nil
	}
	return nil, fmt.Errorf("known_hosts %q not found and insecure=false", knownHosts)
}

func ParseAuthorizedKeys(lines []string) ([]ssh.PublicKey, error) {
	var out []ssh.PublicKey
	for _, l := range lines {
		k, _, _, _, err := ssh.ParseAuthorizedKey([]byte(l))
		if err != nil {
			return nil, fmt.Errorf("parse authorized key: %w", err)
		}
		out = append(out, k)
	}
	return out, nil
}

func KeyAllowed(got ssh.PublicKey, allowed []ssh.PublicKey) bool {
	gm := got.Marshal()
	for _, a := range allowed {
		if bytes.Equal(gm, a.Marshal()) {
			return true
		}
	}
	return false
}
