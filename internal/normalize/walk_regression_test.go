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

func TestWalkResolvesDotDotBeforeCleaning(t *testing.T) {
	base := t.TempDir()
	mkdirs(t, base, "local", "physical/child")
	touch(t, base, "local/My File.txt", "physical/My File.txt")
	walkSymlink(t, filepath.Join(base, "physical/child"), filepath.Join(base, "local/link"))
	// Join would erase the very component this regression exercises.
	root := filepath.Join(base, "local/link") + "/../My File.txt"
	got, err := Walk([]string{root}, Config{})
	if err != nil {
		t.Fatal(err)
	}
	physical, err := filepath.EvalSymlinks(filepath.Join(base, "physical"))
	if err != nil {
		t.Fatal(err)
	}
	assertPaths(t, got, []string{filepath.Join(physical, "My File.txt")})
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
		got, err := Walk([]string{root}, Config{Recursive: true})
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
		var stderr bytes.Buffer
		got, err := Walk([]string{
			filepath.Join(base, ".git/objects/File.txt"),
			filepath.Join(base, ".DS_Store/child/File.txt"),
			filepath.Join(base, "alias/File.txt"),
		}, Config{Recursive: recursive, ErrWriter: &stderr})
		if err != nil || len(got) != 0 {
			t.Fatalf("Walk = %v, %v; want skipped roots", got, err)
		}
	}
	chdir(t, filepath.Join(base, ".git/objects"))
	got, err := Walk([]string{"File.txt", "."}, Config{Recursive: true})
	if err != nil || len(got) != 0 {
		t.Fatalf("Walk inside .git = %v, %v", got, err)
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
			got, err := Walk([]string{root + "/"}, Config{Recursive: recursive})
			if err != nil {
				t.Fatal(err)
			}
			assertPaths(t, got, []string{root})
		}
	}
}

func TestWalkDotRootsSelectContentsNotDirectoryEntry(t *testing.T) {
	base := t.TempDir()
	mkdirs(t, base, "Parent Dir/Child Dir")
	touch(t, base, "Parent Dir/File.txt", "Parent Dir/Child Dir/Child.txt")
	chdir(t, filepath.Join(base, "Parent Dir/Child Dir"))
	for _, root := range []string{".", ".."} {
		got, err := Walk([]string{root}, Config{})
		if err != nil || len(got) != 0 {
			t.Fatalf("nonrecursive Walk(%q) = %v, %v", root, got, err)
		}
		got, err = Walk([]string{root}, Config{Recursive: true})
		if err != nil {
			t.Fatal(err)
		}
		wantCount := 1
		if root == ".." {
			wantCount = 3
		}
		if len(got) != wantCount {
			t.Fatalf("Walk(%q) = %v; want %d contents", root, got, wantCount)
		}
		for _, p := range got {
			if p == root || filepath.Base(p) == "Parent Dir" {
				t.Fatalf("dot root exposed as renameable entry: %q", p)
			}
		}
	}
}

func TestWalkDeduplicatesPhysicalRecursiveSubtrees(t *testing.T) {
	base := t.TempDir()
	mkdirs(t, base, "tree/sub")
	touch(t, base, "tree/sub/File.txt", "tree/Other.txt")
	walkSymlink(t, filepath.Join(base, "tree"), filepath.Join(base, "alias"))
	root := filepath.Join(base, "tree")
	sub := filepath.Join(root, "sub")
	aliasSub := filepath.Join(base, "alias/sub")
	for _, roots := range [][]string{
		{root, root, sub, aliasSub},
		{aliasSub, aliasSub, root, sub},
	} {
		got, err := Walk(roots, Config{Recursive: true})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 4 {
			t.Fatalf("Walk(%v) = %v; physical subtree traversed more than once", roots, got)
		}
	}
}

func TestWalkDotDotDirectoryUsesPhysicalContents(t *testing.T) {
	base := t.TempDir()
	mkdirs(t, base, "local", "Physical Parent/child")
	touch(t, base, "Physical Parent/File.txt", "local/Wrong.txt")
	walkSymlink(t, filepath.Join(base, "Physical Parent/child"), filepath.Join(base, "local/link"))
	got, err := Walk([]string{filepath.Join(base, "local/link") + "/.."}, Config{Recursive: true})
	if err != nil {
		t.Fatal(err)
	}
	physical, err := filepath.EvalSymlinks(filepath.Join(base, "Physical Parent"))
	if err != nil {
		t.Fatal(err)
	}
	assertPaths(t, got, []string{filepath.Join(physical, "File.txt"), filepath.Join(physical, "child")})
}

func TestWalkFailureDoesNotCompleteSubtrees(t *testing.T) {
	base := t.TempDir()
	mkdirs(t, base, "root/accessible", "root/locked")
	touch(t, base, "root/accessible/File.txt")
	locked := filepath.Join(base, "root/locked")
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0o755) })
	if _, err := os.ReadDir(locked); err == nil {
		t.Skip("directory permissions do not restrict this user")
	}
	state := newWalkState()
	_, err := walkRootWithCompleted(filepath.Join(base, "root"), Config{Recursive: true}, state)
	if err == nil || len(state.completed) != 0 || len(state.emitted) != 0 {
		t.Fatalf("failed walk = %v, state = %+v", err, state)
	}
	got, err := walkRootWithCompleted(filepath.Join(base, "root/accessible"), Config{Recursive: true}, state)
	if err != nil || len(got) != 2 {
		t.Fatalf("retry accessible subtree = %v, %v", got, err)
	}
}

func TestWalkCompletedContentsStillEmitDirectoryEntry(t *testing.T) {
	for _, tc := range []struct {
		name  string
		roots []string
		dir   string
		want  []string
		final string
	}{
		{"root", []string{"Dir/.", "Dir"}, "Dir", []string{"Dir", "Dir/File.txt"}, "dir/file.txt"},
		{"child", []string{"Parent/Child/.", "Parent"}, "Parent/Child", []string{"Parent", "Parent/Child", "Parent/Child/File.txt"}, "parent/child/file.txt"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := t.TempDir()
			mkdirs(t, base, tc.dir)
			touch(t, base, tc.dir+"/File.txt")
			chdir(t, base)
			var output bytes.Buffer
			config := Config{Recursive: true, Writer: &output, ErrWriter: &output}
			got, err := Walk(tc.roots, config)
			if err != nil {
				t.Fatal(err)
			}
			want := make([]string, len(tc.want))
			for i, p := range tc.want {
				want[i] = filepath.FromSlash(p)
			}
			assertPaths(t, got, want)
			if err := Run(tc.roots, config); err != nil {
				t.Fatal(err)
			}
			exists(t, base, tc.final)
		})
	}
}

func TestWalkDeduplicatesNativeCaseAliases(t *testing.T) {
	base := t.TempDir()
	mkdirs(t, base, "Tree/Child")
	touch(t, base, "Tree/Child/File.txt")
	root := filepath.Join(base, "Tree")
	alias := filepath.Join(base, "tREE")
	info, err := os.Stat(root)
	if err != nil {
		t.Fatal(err)
	}
	aliasInfo, err := os.Stat(alias)
	if os.IsNotExist(err) {
		t.Skip("filesystem is case-sensitive")
	}
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(info, aliasInfo) {
		t.Skip("case variant is not an alias")
	}
	for _, roots := range [][]string{
		{root, alias},
		{alias, root},
		{filepath.Join(alias, "Child"), root},
		{alias + string(filepath.Separator) + ".", root},
	} {
		got, err := Walk(roots, Config{Recursive: true})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 3 {
			t.Fatalf("Walk(%v) = %v; want each directory and file once", roots, got)
		}
		seen := make(map[string]bool)
		for _, p := range got {
			key := strings.ToLower(p)
			if seen[key] {
				t.Fatalf("Walk(%v) emitted case aliases: %v", roots, got)
			}
			seen[key] = true
		}
	}
}

func TestWalkDeduplicatesExactRecursiveRootsBeforeResolution(t *testing.T) {
	root := filepath.Join(t.TempDir(), "missing")
	_, err := Walk([]string{root, root}, Config{Recursive: true})
	if err == nil || strings.Count(err.Error(), "fpn:") != 1 {
		t.Fatalf("expected one failed resolution for repeated root, got %v", err)
	}
}
