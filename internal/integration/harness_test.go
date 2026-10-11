package integration

import (
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/teawithsand/nworm/internal/agent"
	"github.com/teawithsand/nworm/internal/config"
	"github.com/teawithsand/nworm/internal/keyutil"
	"github.com/teawithsand/nworm/internal/relay"
	"github.com/teawithsand/nworm/internal/sshx"
)

type bench struct {
	relayAddr  string
	echoPort   int
	dir        string
	knownHosts string

	adminKey string
	emptyKey string
	otherKey string
	starKey  string
}

func keyLine(k ssh.PublicKey) string { return string(ssh.MarshalAuthorizedKey(k)) }

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func waitTCP(t *testing.T, addr string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("tcp", addr, 300*time.Millisecond)
		if err == nil {
			c.Close()
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("no listener on %s", addr)
}

func echoLoop(ln net.Listener) {
	for {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		go func(c net.Conn) { defer c.Close(); _, _ = io.Copy(c, c) }(c)
	}
}

type admitFunc func(admin, empty, star ssh.PublicKey) map[string]config.AdminACL

func startBench(t *testing.T, admins admitFunc) *bench {
	t.Helper()
	dir := t.TempDir()

	hostKey := filepath.Join(dir, "host.key")
	if _, err := keyutil.LoadOrCreateHostKey(hostKey); err != nil {
		t.Fatal(err)
	}
	agentPub, err := keyutil.GenerateIdentity(filepath.Join(dir, "agent.key"), "agent@test")
	if err != nil {
		t.Fatal(err)
	}
	adminPub, err := keyutil.GenerateIdentity(filepath.Join(dir, "admin.key"), "admin@test")
	if err != nil {
		t.Fatal(err)
	}
	emptyPub, err := keyutil.GenerateIdentity(filepath.Join(dir, "empty.key"), "empty@test")
	if err != nil {
		t.Fatal(err)
	}
	starPub, err := keyutil.GenerateIdentity(filepath.Join(dir, "star.key"), "star@test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := keyutil.GenerateIdentity(filepath.Join(dir, "other.key"), "other@test"); err != nil {
		t.Fatal(err)
	}
	hostPub, err := os.ReadFile(hostKey + ".pub")
	if err != nil {
		t.Fatal(err)
	}

	echoLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { echoLn.Close() })
	echoPort := echoLn.Addr().(*net.TCPAddr).Port
	go echoLoop(echoLn)

	relayPort := freePort(t)
	relayAddr := "127.0.0.1:" + strconv.Itoa(relayPort)
	knownHosts := filepath.Join(dir, "known_hosts")
	entry := "[127.0.0.1]:" + strconv.Itoa(relayPort) + " " + string(hostPub)
	if err := os.WriteFile(knownHosts, []byte(entry), 0o600); err != nil {
		t.Fatal(err)
	}

	if admins == nil {
		admins = func(admin, _, _ ssh.PublicKey) map[string]config.AdminACL {
			return map[string]config.AdminACL{
				"admin": {Keys: []string{keyLine(admin)}, Allow: []string{"test"}},
			}
		}
	}

	relayCfg := &config.Relay{
		Listen:  relayAddr,
		HostKey: hostKey,
		Agents:  map[string]config.AgentACL{"test": {Keys: []string{keyLine(agentPub)}}},
		Admins:  admins(adminPub, emptyPub, starPub),
	}
	go func() { _ = relay.Run(relayCfg) }()
	waitTCP(t, relayAddr)

	agentCfg := &config.Agent{
		Name:       "test",
		Identity:   filepath.Join(dir, "agent.key"),
		Relays:     []string{relayAddr},
		Remote:     "127.0.0.1:" + strconv.Itoa(echoPort),
		Local:      "127.0.0.1:" + strconv.Itoa(echoPort),
		KnownHosts: knownHosts,
	}
	go func() { _ = agent.Run(agentCfg) }()

	return &bench{
		relayAddr:  relayAddr,
		echoPort:   echoPort,
		dir:        dir,
		knownHosts: knownHosts,
		adminKey:   filepath.Join(dir, "admin.key"),
		emptyKey:   filepath.Join(dir, "empty.key"),
		otherKey:   filepath.Join(dir, "other.key"),
		starKey:    filepath.Join(dir, "star.key"),
	}
}

func (b *bench) dial(t *testing.T, user, keyPath string) (*ssh.Client, error) {
	t.Helper()
	signer, err := keyutil.LoadSigner(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	cb, err := keyutil.HostKeyCallback(b.knownHosts, false)
	if err != nil {
		t.Fatal(err)
	}
	return sshx.Dial(b.relayAddr, &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: cb,
		Timeout:         5 * time.Second,
	})
}

func (b *bench) waitReachable(t *testing.T, user, keyPath string) {
	t.Helper()
	deadline := time.Now().Add(12 * time.Second)
	for time.Now().Before(deadline) {
		c, err := b.dial(t, user, keyPath)
		if err == nil {
			conn, derr := c.Dial("tcp", "test:"+strconv.Itoa(b.echoPort))
			if derr == nil {
				conn.Close()
				c.Close()
				return
			}
			c.Close()
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("bench: agent not reachable")
}
