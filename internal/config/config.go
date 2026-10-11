package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Duration time.Duration

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}
	if s == "" {
		*d = 0
		return nil
	}
	v, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	*d = Duration(v)
	return nil
}

func (d Duration) D() time.Duration { return time.Duration(d) }

type AgentACL struct {
	Keys []string `yaml:"keys"`
}

type AdminACL struct {
	Keys  []string `yaml:"keys"`
	Allow []string `yaml:"allow"`
}

type Relay struct {
	Listen    string              `yaml:"listen"`
	HostKey   string              `yaml:"host_key"`
	Keepalive Duration            `yaml:"keepalive"`
	Agents    map[string]AgentACL `yaml:"agents"`
	Admins    map[string]AdminACL `yaml:"admins"`
}

type Agent struct {
	Name       string   `yaml:"name"`
	Identity   string   `yaml:"identity"`
	Relays     []string `yaml:"relays"`
	Remote     string   `yaml:"remote"`
	Local      string   `yaml:"local"`
	KnownHosts string   `yaml:"known_hosts"`
	Insecure   bool     `yaml:"insecure"`
	Keepalive  Duration `yaml:"keepalive"`
}

type Connect struct {
	AdminUser  string            `yaml:"admin_user"`
	Identity   string            `yaml:"identity"`
	KnownHosts string            `yaml:"known_hosts"`
	Insecure   bool              `yaml:"insecure"`
	Relays     []string          `yaml:"relays"`
	Port       uint32            `yaml:"port"`
	Dest       map[string]uint32 `yaml:"dest"`
}

func Load(path string, out any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(b, out)
}
