package util

import (
	"runtime"
	"sort"
	"strings"
)

// BuildChildEnv builds the environment ("KEY=VALUE" slice) for a child process
// launched on behalf of a script. It inherits from the host ONLY the variables
// whose names are in allow — an explicit allowlist, NOT the host's full
// os.Environ(), which would leak every host secret (API keys, tokens) and
// LD_PRELOAD-style injection vectors into a script-spawned process — then
// applies extra (script-supplied additions/overrides). A nil/empty allow
// inherits nothing from the host. The result is sorted for determinism.
//
// On Windows, environment variable names are case-insensitive, so an extra
// override reliably beats a case-variant host var ("PATH" vs "Path").
func BuildChildEnv(hostEnv []string, allow []string, extra map[string]string) []string {
	return buildChildEnv(runtime.GOOS, hostEnv, allow, extra)
}

func buildChildEnv(goos string, hostEnv []string, allow []string, extra map[string]string) []string {
	// fold canonicalizes a variable name for de-duplication: identity on
	// case-sensitive platforms, upper-case on Windows.
	fold := func(s string) string { return s }
	if goos == "windows" {
		fold = strings.ToUpper
	}

	allowed := make(map[string]struct{}, len(allow))
	for _, k := range allow {
		allowed[fold(k)] = struct{}{}
	}
	// out is keyed by folded name; name keeps the display casing (extra's casing
	// wins because it is applied last).
	out := make(map[string]string, len(allow)+len(extra))
	name := make(map[string]string, len(allow)+len(extra))
	for _, kv := range hostEnv {
		if i := strings.IndexByte(kv, '='); i > 0 {
			if _, ok := allowed[fold(kv[:i])]; ok {
				out[fold(kv[:i])] = kv[i+1:]
				name[fold(kv[:i])] = kv[:i]
			}
		}
	}
	// Iterate extra in sorted key order so that two case-variant keys that fold
	// together (e.g. "Path" and "PATH" on Windows) resolve to a deterministic
	// winner instead of depending on map iteration order.
	extraKeys := make([]string, 0, len(extra))
	for k := range extra {
		extraKeys = append(extraKeys, k)
	}
	sort.Strings(extraKeys)
	for _, k := range extraKeys {
		out[fold(k)] = extra[k]
		name[fold(k)] = k
	}
	keys := make([]string, 0, len(out))
	for k := range out {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	res := make([]string, 0, len(keys))
	for _, k := range keys {
		res = append(res, name[k]+"="+out[k])
	}
	return res
}
