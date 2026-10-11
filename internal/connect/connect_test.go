package connect

import (
	"testing"

	"github.com/teawithsand/nworm/internal/config"
)

func TestResolvePortPrecedence(t *testing.T) {
	cases := []struct {
		name   string
		cfg    *config.Connect
		target string
		flag   uint32
		want   uint32
	}{
		{"explicit flag wins", &config.Connect{Port: 2222, Dest: map[string]uint32{"home": 3333}}, "home", 1111, 1111},
		{"dest beats port", &config.Connect{Port: 2222, Dest: map[string]uint32{"home": 3333}}, "home", 0, 3333},
		{"port beats default", &config.Connect{Port: 2222}, "home", 0, 2222},
		{"default is 22", &config.Connect{}, "home", 0, 22},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := resolvePort(c.cfg, c.target, c.flag); got != c.want {
				t.Fatalf("resolvePort = %d, want %d", got, c.want)
			}
		})
	}
}
