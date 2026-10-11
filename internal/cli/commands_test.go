package cli

import "testing"

func TestCommandHelpDoesNotError(t *testing.T) {
	for _, cmd := range []string{"relay", "agent", "connect", "keygen", "version"} {
		if code := Run(NewRoot(), []string{cmd, "--help"}); code != 0 {
			t.Fatalf("%s --help exit code = %d, want 0", cmd, code)
		}
	}
}

func TestConnectRequiresName(t *testing.T) {
	if code := Run(NewRoot(), []string{"connect"}); code != 2 {
		t.Fatalf("connect without name exit code = %d, want 2", code)
	}
}

func TestKeygenRejectsExtraArgs(t *testing.T) {
	if code := Run(NewRoot(), []string{"keygen", "extra"}); code != 2 {
		t.Fatalf("keygen with extra arg exit code = %d, want 2", code)
	}
}
