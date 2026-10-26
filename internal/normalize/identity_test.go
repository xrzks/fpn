package normalize

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDirectoryKey(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "directory")
	other := filepath.Join(root, "other")
	for _, path := range []string{dir, other} {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	key := func(t *testing.T, path string) string {
		t.Helper()
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		value, err := directoryKey(path, info)
		if err != nil {
			t.Fatal(err)
		}
		if value == "" {
			t.Fatal("empty directory key")
		}
		return value
	}
	want := key(t, dir)
	if got := key(t, other); got == want {
		t.Fatal("distinct directories have the same key")
	}

	t.Run("absolute", func(t *testing.T) {
		absolute, err := filepath.Abs(dir)
		if err != nil {
			t.Fatal(err)
		}
		cwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		relative, err := filepath.Rel(cwd, absolute)
		if err != nil {
			t.Skipf("cannot make relative path: %v", err)
		}
		if got := key(t, relative); got != key(t, absolute) {
			t.Fatalf("relative and absolute keys differ: %q != %q", got, want)
		}
	})

	t.Run("symlink", func(t *testing.T) {
		link := filepath.Join(root, "link")
		if err := os.Symlink(dir, link); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
		if got := key(t, link); got != want {
			t.Fatalf("symlink key = %q, want %q", got, want)
		}
	})
}
