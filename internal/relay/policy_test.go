package relay

import "testing"

func TestACLAllowed(t *testing.T) {
	cases := []struct {
		name string
		acl  *acl
		host string
		want bool
	}{
		{"nil acl denies", nil, "home", false},
		{"empty allow denies everything", &acl{allow: map[string]bool{}}, "home", false},
		{"explicit host is allowed", &acl{allow: map[string]bool{"home": true}}, "home", true},
		{"different host is denied", &acl{allow: map[string]bool{"home": true}}, "office", false},
		{"wildcard allows everything", &acl{allow: map[string]bool{"*": true}}, "anything", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.acl.allowed(c.host); got != c.want {
				t.Fatalf("allowed(%q) = %v, want %v", c.host, got, c.want)
			}
		})
	}
}
