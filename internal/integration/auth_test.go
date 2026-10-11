package integration

import (
	"strconv"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"

	"github.com/teawithsand/nworm/internal/config"
)

func keyText(k ssh.PublicKey) string { return string(ssh.MarshalAuthorizedKey(k)) }

func TestWrongAdminKeyRejected(t *testing.T) {
	b := startBench(t, nil)
	// user "admin" exists, but other.key is not in the relay's authorized keys.
	if _, err := b.dial(t, "admin", b.otherKey); err == nil {
		t.Fatal("expected handshake to be rejected for an unauthorized key")
	}
}

func TestUnknownAgentNameRejected(t *testing.T) {
	// Use an admin allowed to reach everything, so the request passes the
	// allow check and exercises the agent-lookup path.
	b := startBench(t, func(admin, empty, star ssh.PublicKey) map[string]config.AdminACL {
		return map[string]config.AdminACL{
			"admin-star": {Keys: []string{keyText(star)}, Allow: []string{"*"}},
		}
	})
	b.waitReachable(t, "admin-star", b.starKey)

	client, err := b.dial(t, "admin-star", b.starKey)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	if _, err := client.Dial("tcp", "nosuch:22"); err == nil {
		t.Fatal("expected unknown agent name to be rejected")
	} else if !strings.Contains(err.Error(), "no agent registered") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEmptyAllowDeniesAll(t *testing.T) {
	b := startBench(t, func(admin, empty, star ssh.PublicKey) map[string]config.AdminACL {
		return map[string]config.AdminACL{
			"admin":       {Keys: []string{keyText(admin)}, Allow: []string{"test"}},
			"admin-empty": {Keys: []string{keyText(empty)}}, // deliberately no allow
		}
	})
	b.waitReachable(t, "admin", b.adminKey)

	client, err := b.dial(t, "admin-empty", b.emptyKey)
	if err != nil {
		t.Fatalf("admin-empty should authenticate but was rejected: %v", err)
	}
	defer client.Close()

	if _, err := client.Dial("tcp", "test:"+strconv.Itoa(b.echoPort)); err == nil {
		t.Fatal("SECURITY: an admin with an empty allow list reached the agent (fail-open)")
	} else if !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("expected an authorization rejection, got: %v", err)
	}
}

func TestWildcardAllowPermitsAll(t *testing.T) {
	b := startBench(t, func(admin, empty, star ssh.PublicKey) map[string]config.AdminACL {
		return map[string]config.AdminACL{
			"admin-star": {Keys: []string{keyText(star)}, Allow: []string{"*"}},
		}
	})
	b.waitReachable(t, "admin-star", b.starKey)
}
