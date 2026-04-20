// Package envutil provides small helpers for manipulating environment
// variable slices (the os.Environ() "KEY=VALUE" string form).
//
// Kept separate from internal/config and internal/project so packages that
// only need env munging don't pull in config file or filesystem-walk code.
package envutil

import "strings"

// Strip returns env with any entries whose key matches one of the given keys
// removed. Match is on the "KEY=" prefix, so similarly-prefixed keys (e.g.
// "PATH" vs "PATHEXT") are not confused. Order of remaining entries is
// preserved.
func Strip(env []string, keys ...string) []string {
	if len(keys) == 0 {
		return env
	}
	prefixes := make([]string, len(keys))
	for i, k := range keys {
		prefixes[i] = k + "="
	}
	out := make([]string, 0, len(env))
outer:
	for _, entry := range env {
		for _, p := range prefixes {
			if strings.HasPrefix(entry, p) {
				continue outer
			}
		}
		out = append(out, entry)
	}
	return out
}
