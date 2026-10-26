package normalize

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func walkSymlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}
}

func physicalDir(t *testing.T, path string) string {
	t.Helper()
	physical, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return physical
}

func TestWalkRejectsEmptyRoot(t *testing.T) {
	for _, recursive := range []bool{false, true} {
		got, err := Walk("", Config{Recursive: recursive})
		if err == nil || !strings.Contains(err.Error(), "path argument must not be empty") || len(got) != 0 {
			t.Fatalf("Walk(empty, recursive=%v) = %v, %v", recursive, got, err)
		}
	}
}

func TestWalkResolvesDotDotBeforeCleaning(t *testing.T) {
	base := t.TempDir()
	mkdirs(t, base, "local", "physical/child")
	touch(t, base, "local/My File.txt", "physical/My File.txt")
	walkSymlink(t, filepath.Join(base, "physical/child"), filepath.Join(base, "local/link"))
	// Join would erase the very component this regression exercises.
	root := filepath.Join(base, "local/link") + "/../My File.txt"
	got, err := Walk(root, Config{})
	if err != nil {
		t.Fatal(err)
	}
	assertPaths(t, got, []string{filepath.Join(physicalDir(t, filepath.Join(base, "physical")), "My File.txt")})
	if err := NormalizePaths(got, Config{}); err != nil {
		t.Fatal(err)
	}
	exists(t, base, "local/My File.txt", "physical/my-file.txt")
	missing(t, base, "local/my-file.txt", "physical/My File.txt")
}

func TestWalkRejectsMissingOrNonDirectoryBeforeDotDot(t *testing.T) {
	base := t.TempDir()
	touch(t, base, "File.txt", "not-a-directory")
	for _, component := range []string{"missing", "not-a-directory"} {
		root := base + "/" + component + "/../File.txt"
		got, err := Walk(root, Config{Recursive: true})
		if err == nil || len(got) != 0 {
			t.Fatalf("Walk(%q) = %v, %v; want no paths and an error", root, got, err)
		}
	}
}

func TestWalkSkipsExplicitAndResolvedAncestors(t *testing.T) {
	base := t.TempDir()
	mkdirs(t, base, ".git/objects", ".DS_Store/child")
	touch(t, base, ".git/objects/File.txt", ".DS_Store/child/File.txt")
	walkSymlink(t, filepath.Join(base, ".git/objects"), filepath.Join(base, "alias"))
	for _, recursive := range []bool{false, true} {
		for _, root := range []string{
			filepath.Join(base, ".git/objects/File.txt"),
			filepath.Join(base, ".DS_Store/child/File.txt"),
			filepath.Join(base, "alias/File.txt"),
		} {
			var stderr bytes.Buffer
			got, err := Walk(root, Config{Recursive: recursive, ErrWriter: &stderr})
			if err != nil || len(got) != 0 || !strings.Contains(stderr.String(), "skipped") {
				t.Fatalf("Walk(%q) = %v, %v, diagnostics=%q", root, got, err, &stderr)
			}
		}
	}
}

func TestWalkFinalSymlinksAreEntriesEvenWithSlash(t *testing.T) {
	base := t.TempDir()
	mkdirs(t, base, "target")
	touch(t, base, "target/File.txt")
	walkSymlink(t, "target", filepath.Join(base, "Link"))
	walkSymlink(t, "missing", filepath.Join(base, "Dangling"))
	for _, name := range []string{"Link", "Dangling"} {
		for _, recursive := range []bool{false, true} {
			root := filepath.Join(base, name)
			got, err := Walk(root+"/", Config{Recursive: recursive})
			if err != nil {
				t.Fatal(err)
			}
			assertPaths(t, got, []string{filepath.Join(physicalDir(t, base), name)})
		}
	}
}

func TestWalkDotRootsSelectFilesOnly(t *testing.T) {
	base := t.TempDir()
	mkdirs(t, base, "Parent Dir/Child Dir")
	touch(t, base, "Parent Dir/File.txt", "Parent Dir/Child Dir/Child.txt")
	chdir(t, filepath.Join(base, "Parent Dir/Child Dir"))
	parent := physicalDir(t, filepath.Join(base, "Parent Dir"))
	for _, root := range []string{".", ".."} {
		got, err := Walk(root, Config{})
		if err != nil || len(got) != 0 {
			t.Fatalf("nonrecursive Walk(%q) = %v, %v", root, got, err)
		}
		got, err = Walk(root, Config{Recursive: true})
		if err != nil {
			t.Fatal(err)
		}
		want := []string{filepath.Join(parent, "Child Dir/Child.txt")}
		if root == ".." {
			want = append(want, filepath.Join(parent, "File.txt"))
		}
		assertPaths(t, got, want)
	}
}

func TestWalkRelativeRootReturnsPhysicalAbsolutePaths(t *testing.T) {
	base := t.TempDir()
	mkdirs(t, base, "Physical Dir/Child Dir", "Physical Dir/.git")
	touch(t, base, "Physical Dir/Child Dir/File.txt", "Physical Dir/.git/Hidden.txt", "Physical Dir/.DS_Store")
	walkSymlink(t, "Physical Dir", filepath.Join(base, "alias"))
	walkSymlink(t, "missing", filepath.Join(base, "Physical Dir/Link"))
	chdir(t, base)
	var stderr bytes.Buffer
	got, err := Walk("alias/Child Dir/..", Config{Recursive: true, ErrWriter: &stderr})
	if err != nil {
		t.Fatal(err)
	}
	physical := physicalDir(t, filepath.Join(base, "Physical Dir"))
	assertPaths(t, got, []string{filepath.Join(physical, "Child Dir/File.txt"), filepath.Join(physical, "Link")})
	if stderr.Len() != 0 {
		t.Fatalf("recursive exclusions should be silent: %s", &stderr)
	}
	got, err = Walk("alias/Child Dir", Config{})
	if err != nil {
		t.Fatal(err)
	}
	assertPaths(t, got, []string{filepath.Join(physical, "Child Dir")})
}

func TestRunWalkFailureDoesNotRenameFiles(t *testing.T) {
	base := t.TempDir()
	mkdirs(t, base, "Root Dir/locked")
	touch(t, base, "Root Dir/A File.txt")
	locked := filepath.Join(base, "Root Dir/locked")
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
	if _, err := os.ReadDir(locked); err == nil {
		t.Skip("directory permissions do not restrict this user")
	}
	var out bytes.Buffer
	err := Run([]string{filepath.Join(base, "Root Dir")}, Config{Recursive: true, Writer: &out})
	if err == nil || out.Len() != 0 {
		t.Fatalf("Run = %v, output=%q", err, &out)
	}
	exists(t, base, "Root Dir/A File.txt")
	missing(t, base, "Root Dir/a-file.txt")
}
