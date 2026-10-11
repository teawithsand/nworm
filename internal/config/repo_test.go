package config

import (
	"os"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestRepoYAMLParses guards the example configs and the CI/dependabot YAML
// against syntax breakage. Paths are relative to this package directory.
func TestRepoYAMLParses(t *testing.T) {
	paths := []string{
		"../../examples/relay.yaml",
		"../../examples/agent.yaml",
		"../../examples/connect.yaml",
		"../../.github/workflows/ci.yml",
		"../../.github/dependabot.yml",
	}
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		var n yaml.Node
		if err := yaml.Unmarshal(b, &n); err != nil {
			t.Fatalf("%s: %v", p, err)
		}
	}
}
