package util

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ResolveUnder resolves userPath against root and returns the confined path ONLY
// if it stays within root — a path jail. It confines "..", absolute paths, and
// symlinks (including dangling ones) in ANY path segment whose target escapes
// root, so a script-supplied path cannot reach an arbitrary host file (the
// traversal / arbitrary read-write class of bug). userPath is treated as
// relative to root: an absolute userPath is re-anchored under root.
//
// The check is time-of-check: it validates and returns a path the caller opens
// later. It is safe when path components under root cannot be concurrently
// replaced by a hostile party; if the tree is attacker-writable between the call
// and the open, a TOCTOU race remains (open the returned path with O_NOFOLLOW,
// or use an OS-level jail, for that threat model).
func ResolveUnder(root, userPath string) (string, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	joined := filepath.Join(rootAbs, filepath.FromSlash(userPath))
	if !withinRoot(rootAbs, joined) {
		return "", fmt.Errorf("path %q escapes root %q", userPath, root)
	}
	// Resolve the root once (it may itself sit under a symlink, e.g. /var ->
	// /private/var); walk the remainder from there so every comparison is
	// symlink-free on the root side.
	realRoot, err := filepath.EvalSymlinks(rootAbs)
	if err != nil {
		realRoot = rootAbs // root does not exist yet; walk from its abs path
	}
	rel, err := filepath.Rel(rootAbs, joined)
	if err != nil {
		return "", err
	}
	// Descend component by component, resolving any symlink segment (via Lstat +
	// Readlink so a DANGLING link is caught too, unlike EvalSymlinks which errors
	// on it) and re-confining to realRoot at each step. The first non-existent
	// component ends the walk: the rest is a not-yet-created path with no links.
	cur := realRoot
	for _, seg := range strings.Split(rel, string(os.PathSeparator)) {
		if seg == "" || seg == "." {
			continue
		}
		if seg == ".." { // defensive: Join already collapsed these
			return "", fmt.Errorf("path %q escapes root %q", userPath, root)
		}
		cur = filepath.Join(cur, seg)
		fi, err := os.Lstat(cur)
		if err != nil {
			continue // does not exist yet
		}
		if fi.Mode()&os.ModeSymlink == 0 {
			continue // a real dir/file, stays put
		}
		target, err := os.Readlink(cur)
		if err != nil {
			return "", err
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(cur), target)
		}
		target = resolveExistingPrefix(filepath.Clean(target))
		if !withinRoot(realRoot, target) {
			return "", fmt.Errorf("path %q resolves outside root %q via a symlink", userPath, root)
		}
		cur = target
	}
	return cur, nil
}

// resolveExistingPrefix returns the real (symlink-resolved) path of the deepest
// existing ancestor of p (p itself when it exists).
func resolveExistingPrefix(p string) string {
	for {
		if real, err := filepath.EvalSymlinks(p); err == nil {
			return real
		}
		parent := filepath.Dir(p)
		if parent == p { // reached the filesystem root
			return p
		}
		p = parent
	}
}

// withinRoot reports whether p is rootAbs or a path beneath it.
func withinRoot(rootAbs, p string) bool {
	pAbs, err := filepath.Abs(p)
	if err != nil {
		return false
	}
	if pAbs == rootAbs {
		return true
	}
	// Use rootAbs + separator as the boundary so "/rootfoo" is not treated as
	// under "/root"; avoid a doubled separator when rootAbs already ends in one
	// (a filesystem root like "/" or "C:\").
	sep := string(os.PathSeparator)
	prefix := rootAbs
	if !strings.HasSuffix(prefix, sep) {
		prefix += sep
	}
	return strings.HasPrefix(pAbs, prefix)
}
