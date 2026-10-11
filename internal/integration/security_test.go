package integration

import (
	"strings"
	"testing"
)

// The relay must expose exactly one channel type per role. An admin is only
// allowed to open direct-tcpip channels; anything else is a privilege
// escalation attempt.
func TestAdminCannotOpenSessionChannel(t *testing.T) {
	b := startBench(t, nil)
	b.waitReachable(t, "admin", b.adminKey)

	client, err := b.dial(t, "admin", b.adminKey)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	if _, _, err := client.OpenChannel("session", nil); err == nil {
		t.Fatal("SECURITY: relay accepted a session channel from an admin (only direct-tcpip should be allowed)")
	}
}

// An admin may only reach ports that the agent actually registered.
func TestUnregisteredPortRejected(t *testing.T) {
	b := startBench(t, nil)
	b.waitReachable(t, "admin", b.adminKey)

	client, err := b.dial(t, "admin", b.adminKey)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	if _, err := client.Dial("tcp", "test:1"); err == nil {
		t.Fatal("expected an unregistered target port to be rejected")
	} else if !strings.Contains(err.Error(), "no listener on port") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// An unknown agent name must fail at authentication, before any routing.
func TestUnknownAgentUserRejected(t *testing.T) {
	b := startBench(t, nil)
	if _, err := b.dial(t, "agent-nosuch", b.otherKey); err == nil {
		t.Fatal("expected an unknown agent user to be rejected during authentication")
	}
}

// A known agent name with a key that is not in its allow list must be rejected.
func TestWrongAgentKeyRejected(t *testing.T) {
	b := startBench(t, nil)
	if _, err := b.dial(t, "agent-test", b.otherKey); err == nil {
		t.Fatal("expected an unauthorized agent key to be rejected during authentication")
	}
}
