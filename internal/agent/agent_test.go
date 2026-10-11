package agent

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/teawithsand/nworm/internal/config"
)

func TestRunValidation(t *testing.T) {
	idPath := filepath.Join(t.TempDir(), "missing.key")
	cases := []struct {
		name    string
		cfg     *config.Agent
		wantErr string
	}{
		{"missing name", &config.Agent{}, "name is required"},
		{"missing identity", &config.Agent{Name: "home"}, "identity is required"},
		{"missing relays", &config.Agent{Name: "home", Identity: idPath}, "at least one relay"},
		{
			name:    "unreadable identity",
			cfg:     &config.Agent{Name: "home", Identity: idPath, Relays: []string{"127.0.0.1:1"}},
			wantErr: "identity:",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := Run(c.cfg)
			if err == nil {
				t.Fatal("expected Run to fail during validation")
			}
			if !strings.Contains(err.Error(), c.wantErr) {
				t.Fatalf("error = %q, want substring %q", err, c.wantErr)
			}
		})
	}
}
