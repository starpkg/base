package util

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func TestResolveUnder(t *testing.T) {
	root := t.TempDir()
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}

	// a normal in-root path resolves under the (real) root
	got, err := ResolveUnder(root, "sub/file.txt")
	if err != nil {
		t.Fatalf("in-root path errored: %v", err)
	}
	if want := filepath.Join(realRoot, "sub", "file.txt"); got != want {
		t.Errorf("ResolveUnder = %q, want %q", got, want)
	}

	// "." is the root itself
	if got, err := ResolveUnder(root, "."); err != nil || got != realRoot {
		t.Errorf(`ResolveUnder(".") = %q err=%v, want %q`, got, err, realRoot)
	}

	// a not-yet-existing root falls back to its lexical (abs) path
	ghost := filepath.Join(realRoot, "does-not-exist-yet")
	if got, err := ResolveUnder(ghost, "child"); err != nil || got != filepath.Join(ghost, "child") {
		t.Errorf("non-existent root: got=%q err=%v", got, err)
	}

	// traversal escapes are rejected
	if _, err := ResolveUnder(root, "../../etc/passwd"); err == nil {
		t.Error("traversal path should be rejected")
	}

	// an absolute path is re-anchored under root, not honored as-is
	got, err = ResolveUnder(root, "/etc/passwd")
	if err != nil {
		t.Fatalf("absolute path should be confined, not errored: %v", err)
	}
	if want := filepath.Join(realRoot, "etc", "passwd"); got != want {
		t.Errorf("absolute re-anchor = %q, want %q", got, want)
	}

	if runtime.GOOS == "windows" {
		return // symlink semantics/permissions differ on windows CI
	}
	outside := t.TempDir()
	realOutside, _ := filepath.EvalSymlinks(outside)
	sl := func(target, name string) {
		if err := os.Symlink(target, filepath.Join(root, name)); err != nil {
			t.Skipf("cannot create symlink: %v", err)
		}
	}

	// (1) a symlink whose target escapes root — leaf itself is the symlink
	sl(realOutside, "escape")
	if _, err := ResolveUnder(root, "escape"); err == nil {
		t.Error("symlink escaping root should be rejected")
	}
	// (2) an escaping symlink in a NON-final segment with a missing leaf
	if _, err := ResolveUnder(root, "escape/newleaf"); err == nil {
		t.Error("missing leaf under an escaping symlink segment should be rejected")
	}
	// (3) a DANGLING symlink pointing outside (target does not exist)
	sl(filepath.Join(realOutside, "ghost"), "dangling")
	if _, err := ResolveUnder(root, "dangling"); err == nil {
		t.Error("dangling symlink pointing outside root should be rejected")
	}
	// (4) a symlink to root's PARENT then a new leaf (the reverse-containment attack)
	sl(filepath.Dir(realRoot), "up")
	if _, err := ResolveUnder(root, "up/newfile"); err == nil {
		t.Error("symlink to root's parent must not allow escaping via a new leaf")
	}

	// (5) an in-root symlink that stays within root is ALLOWED (relative target,
	// exercising the relative-symlink resolution path)
	if err := os.Mkdir(filepath.Join(realRoot, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	sl("sub", "link")
	if got, err := ResolveUnder(root, "link/x"); err != nil {
		t.Errorf("in-root symlink should be allowed: %v", err)
	} else if want := filepath.Join(realRoot, "sub", "x"); got != want {
		t.Errorf("in-root symlink resolve = %q, want %q", got, want)
	}

	// (7) a symlink CHAIN that ends outside root is caught (chain_a -> chain_b,
	// chain_b -> outside): resolving a segment must follow the whole chain.
	sl(realOutside, "chain_b")
	sl("chain_b", "chain_a")
	if _, err := ResolveUnder(root, "chain_a/x"); err == nil {
		t.Error("a symlink chain ending outside root should be rejected")
	}

	// (8) a symlink reached THROUGH an in-root symlink must still be confined:
	// link -> sub (in-root), sub/evil -> outside; link/evil/x must be rejected,
	// i.e. the walk keeps examining segments AFTER following the first symlink.
	if err := os.Symlink(realOutside, filepath.Join(realRoot, "sub", "evil")); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}
	if _, err := ResolveUnder(root, "link/evil/x"); err == nil {
		t.Error("a symlink out reached through an in-root symlink must be rejected")
	}

	// (6) root that is the filesystem root ("/") must not false-reject children
	if got, err := ResolveUnder("/", "no-such-file-xyz-42"); err != nil || got != "/no-such-file-xyz-42" {
		t.Errorf(`ResolveUnder("/", child) = %q err=%v`, got, err)
	}
}

func TestBuildChildEnv(t *testing.T) {
	host := []string{"PATH=/bin", "SECRET=topsecret", "HOME=/root", "MALFORMED"}

	// only allowlisted host vars survive; the secret is dropped
	got := BuildChildEnv(host, []string{"PATH", "HOME"}, nil)
	want := []string{"HOME=/root", "PATH=/bin"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("allowlist = %v, want %v", got, want)
	}

	// empty allow inherits nothing; only extra is present
	got = BuildChildEnv(host, nil, map[string]string{"FOO": "bar"})
	if !reflect.DeepEqual(got, []string{"FOO=bar"}) {
		t.Errorf("empty allow = %v, want [FOO=bar]", got)
	}

	// extra overrides an allowed host var, and the result is sorted
	got = BuildChildEnv(host, []string{"PATH"}, map[string]string{"PATH": "/override", "AAA": "1"})
	want = []string{"AAA=1", "PATH=/override"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("override+sort = %v, want %v", got, want)
	}

	// on a case-insensitive (windows) platform, an extra override folds together
	// with a case-variant host var so the override wins (single entry).
	winGot := buildChildEnv("windows", []string{"Path=host"}, []string{"Path"}, map[string]string{"PATH": "extra"})
	if !reflect.DeepEqual(winGot, []string{"PATH=extra"}) {
		t.Errorf("windows fold = %v, want [PATH=extra]", winGot)
	}
	// on a case-sensitive (unix) platform they remain distinct.
	nixGot := buildChildEnv("linux", []string{"Path=host"}, []string{"Path"}, map[string]string{"PATH": "extra"})
	if !reflect.DeepEqual(nixGot, []string{"PATH=extra", "Path=host"}) {
		t.Errorf("unix distinct = %v, want [PATH=extra Path=host]", nixGot)
	}
}
