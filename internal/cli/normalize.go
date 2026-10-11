package cli

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// normalizeArgs rewrites single-dash long flags (e.g. -config) to the
// double-dash form pflag requires, so existing invocations keep working.
// Shorthands such as -f and -h are left untouched.
func normalizeArgs(root *cobra.Command, args []string) []string {
	long := map[string]bool{}
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		c.Flags().VisitAll(func(f *pflag.Flag) { long[f.Name] = true })
		c.PersistentFlags().VisitAll(func(f *pflag.Flag) { long[f.Name] = true })
		for _, sub := range c.Commands() {
			walk(sub)
		}
	}
	walk(root)

	out := make([]string, 0, len(args))
	for _, a := range args {
		if !strings.HasPrefix(a, "-") || strings.HasPrefix(a, "--") || len(a) < 2 {
			out = append(out, a)
			continue
		}
		name := strings.TrimPrefix(a, "-")
		if i := strings.IndexByte(name, '='); i >= 0 {
			name = name[:i]
		}
		if long[name] {
			out = append(out, "-"+a)
			continue
		}
		out = append(out, a)
	}
	return out
}
