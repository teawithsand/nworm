package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Options controls how a command's configuration is resolved.
type Options struct {
	// Command is the env segment, e.g. "relay" -> NWORM_RELAY_*.
	Command string
	// ConfigFlag is the value of the --config flag.
	ConfigFlag string
	// Optional allows a missing config file (used by connect).
	Optional bool
	// Aliases binds config keys to differently named flags (key -> flag name).
	Aliases map[string]string
}

// EnvPrefix is prepended to the command segment and the field name, e.g.
// NWORM_RELAY_HOST_KEY for the relay command's host_key field.
const EnvPrefix = "NWORM_"

// Resolve loads configuration with precedence: default < file < env < flag.
func Resolve(cmd *cobra.Command, out any, opts Options) error {
	v := viper.New()
	v.AllowEmptyEnv(true)

	if opts.ConfigFlag != "" {
		v.SetConfigFile(opts.ConfigFlag)
		if err := v.ReadInConfig(); err != nil {
			if !(opts.Optional && errors.Is(err, os.ErrNotExist)) {
				return fmt.Errorf("read config %q: %w", opts.ConfigFlag, err)
			}
		}
	}

	// Flag names use dashes, config keys use underscores: --host-key -> host_key.
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		key := strings.ReplaceAll(f.Name, "-", "_")
		_ = v.BindPFlag(key, f)
	})
	for key, flagName := range opts.Aliases {
		if f := cmd.Flags().Lookup(flagName); f != nil {
			_ = v.BindPFlag(key, f)
		}
	}

	bindEnv(v, opts.Command, fieldKeys(out))

	if err := v.Unmarshal(out,
		viper.DecodeHook(mapstructure.ComposeDecodeHookFunc(
			jsonHook,
			mapstructure.StringToTimeDurationHookFunc(),
			mapstructure.StringToSliceHookFunc(","),
			durationHook,
		)),
		matchNameOption,
	); err != nil {
		return fmt.Errorf("decode config: %w", err)
	}
	return nil
}

func bindEnv(v *viper.Viper, command string, keys []string) {
	if command == "" {
		return
	}
	prefix := EnvPrefix + strings.ToUpper(command)
	for _, key := range keys {
		env := prefix + "_" + strings.ToUpper(key)
		_ = v.BindEnv(key, env)
	}
}

// fieldKeys returns the yaml tag names of the struct pointed to by out.
func fieldKeys(out any) []string {
	t := reflect.TypeOf(out)
	if t == nil || t.Kind() != reflect.Pointer || t.Elem().Kind() != reflect.Struct {
		return nil
	}
	t = t.Elem()
	keys := make([]string, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		name := t.Field(i).Tag.Get("yaml")
		if idx := strings.IndexByte(name, ','); idx >= 0 {
			name = name[:idx]
		}
		if name == "" || name == "-" {
			continue
		}
		keys = append(keys, name)
	}
	return keys
}

// jsonHook lets list and map fields be provided as JSON environment values,
// e.g. NWORM_AGENT_RELAYS='["a:443","b:443"]' or NWORM_CONNECT_DEST='{"home":22}'.
func jsonHook(_ reflect.Type, to reflect.Type, data any) (any, error) {
	s, ok := data.(string)
	if !ok {
		return data, nil
	}
	trimmed := strings.TrimSpace(s)
	if !strings.HasPrefix(trimmed, "[") && !strings.HasPrefix(trimmed, "{") {
		return data, nil
	}
	if to.Kind() != reflect.Slice && to.Kind() != reflect.Map {
		return data, nil
	}
	ptr := reflect.New(to)
	if err := json.Unmarshal([]byte(trimmed), ptr.Interface()); err != nil {
		return nil, fmt.Errorf("parse JSON env value: %w", err)
	}
	return ptr.Elem().Interface(), nil
}

func durationHook(_ reflect.Type, to reflect.Type, data any) (any, error) {
	if to == reflect.TypeOf(Duration(0)) {
		switch v := data.(type) {
		case string:
			if v == "" {
				return Duration(0), nil
			}
			d, err := time.ParseDuration(v)
			if err != nil {
				return nil, err
			}
			return Duration(d), nil
		case int64:
			return Duration(v), nil
		}
	}
	return data, nil
}

// matchNameOption maps viper keys to struct fields ignoring case and separators,
// so "host_key" matches the HostKey field without adding mapstructure tags.
func matchNameOption(config *mapstructure.DecoderConfig) {
	normalize := func(s string) string {
		s = strings.ReplaceAll(s, "_", "")
		s = strings.ReplaceAll(s, "-", "")
		return strings.ToLower(s)
	}
	config.MatchName = func(mapKey, fieldName string) bool {
		return normalize(mapKey) == normalize(fieldName)
	}
}
