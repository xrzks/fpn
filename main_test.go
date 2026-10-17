package main

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

// ---------------------------------------------------------------------------
// normalizePathName
// ---------------------------------------------------------------------------

func TestGetNormalizedPathName(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "spaces and mixed case",
			path: "this is a File.txt",
			want: "this-is-a-file.txt",
		},
		{
			name: "already normalized",
			path: "already-normalized.txt",
			want: "already-normalized.txt",
		},
		{
			name: "all uppercase",
			path: "MY DOCUMENT.PDF",
			want: "my-document.pdf",
		},
		{
			name: "multiple consecutive spaces",
			path: "too   many    spaces.txt",
			want: "too-many-spaces.txt",
		},
		{
			name: "leading and trailing spaces",
			path: "  padded name.txt  ",
			want: "padded-name.txt",
		},
		{
			name: "empty string",
			path: "",
			want: "",
		},
		{
			name: "no extension",
			path: "readme",
			want: "readme",
		},
		{
			name: "single character",
			path: "A",
			want: "a",
		},
		{
			name: "underscores preserved",
			path: "my_file name.txt",
			want: "my-file-name.txt",
		},
		{
			name: "multiple dots in name",
			path: "archive.tar.gz",
			want: "archive.tar.gz",
		},
		{
			name: "numbers in name",
			path: "Report 2024 Q1.xlsx",
			want: "report-2024-q1.xlsx",
		},
		{
			name: "tab and newline whitespace",
			path: "tab\tand\nnewline.txt",
			want: "tab-and-newline.txt",
		},
		{
			name: "mixed whitespace types",
			path: "mixed \t \n spaces.txt",
			want: "mixed-spaces.txt",
		},
		{
			name: "leading and trailing non-space whitespace",
			path: "\t\nfoo\t\n",
			want: "foo",
		},
		{
			name: "whitespace only spaces",
			path: "   ",
			want: "",
		},
		{
			name: "whitespace only tabs and newlines",
			path: "\t\n",
			want: "",
		},
		{
			name: "single space",
			path: " ",
			want: "",
		},
		{
			name: "hyphen between spaces collapses to single hyphen",
			path: "hello -world",
			want: "hello-world",
		},
		{
			name: "spaces around hyphen collapse to single hyphen",
			path: "a  -  b",
			want: "a-b",
		},
		{
			name: "three or more consecutive hyphens collapse to one",
			path: "a---b",
			want: "a-b",
		},
		{
			name: "pre-existing multiple hyphens collapse to one",
			path: "a----b",
			want: "a-b",
		},
		{
			name: "hyphen-space-hyphen becomes single hyphen",
			path: "- -",
			want: "-",
		},
		{
			name: "single hyphens preserved",
			path: "a-b-c",
			want: "a-b-c",
		},
		{
			name: "preserves forward slashes",
			path: "docs/2024/Q1 report.pdf",
			want: "docs/2024/q1-report.pdf",
		},
		{
			name: "unicode accented characters lowercased",
			path: "Café résumé.txt",
			want: "café-résumé.txt",
		},
		{
			name: "punctuation preserved",
			path: "My Report (v2).txt",
			want: "my-report-(v2).txt",
		},
		{
			// Regression: only the final path component may be normalized.
			// os.Rename cannot create missing intermediate directories, so
			// normalizing ancestor directory names (uppercase, spaces,
			// underscores) would produce a target path whose parent
			// directory doesn't exist on disk.
			name: "parent directories are left untouched",
			path: "My_Dir/Sub Folder/My File.txt",
			want: "My_Dir/Sub Folder/my-file.txt",
		},
		{
			name: "trailing slash treated as directory with empty basename",
			path: "My_Dir/",
			want: "My_Dir/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizePathName(tt.path)
			if got != tt.want {
				t.Errorf("getNormalizedPathName(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func mkdirs(t *testing.T, base string, dirs ...string) {
	t.Helper()
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(base, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

func touch(t *testing.T, base string, files ...string) {
	t.Helper()
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(base, f), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// chdir moves into dir for the duration of the test so relative paths work.
func chdir(t *testing.T, dir string) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(old) })
}

func sorted(s []string) []string {
	out := append([]string(nil), s...)
	sort.Strings(out)
	return out
}

func assertPaths(t *testing.T, got, want []string) {
	t.Helper()
	if !reflect.DeepEqual(sorted(got), sorted(want)) {
		t.Fatalf("paths mismatch\n got: %v\nwant: %v", sorted(got), sorted(want))
	}
}

func exists(t *testing.T, base string, rel ...string) {
	t.Helper()
	for _, r := range rel {
		if _, err := os.Lstat(filepath.Join(base, r)); err != nil {
			t.Errorf("expected %s to exist: %v", r, err)
		}
	}
}

func missing(t *testing.T, base string, rel ...string) {
	t.Helper()
	for _, r := range rel {
		if _, err := os.Lstat(filepath.Join(base, r)); err == nil {
			t.Errorf("expected %s not to exist", r)
		}
	}
}

// exampleTree builds the tree from the change spec.
func exampleTree(t *testing.T) string {
	t.Helper()
	base := t.TempDir()
	mkdirs(t, base, "Test Dir/Child One", "Test Dir/Child Two")
	touch(t, base, "Test Dir/Child One/file.txt")
	return base
}

// ---------------------------------------------------------------------------
// Walk
// ---------------------------------------------------------------------------

func TestWalkNonRecursiveDirIsSingleEntry(t *testing.T) {
	base := exampleTree(t)
	root := filepath.Join(base, "Test Dir")
	got, err := Walk([]string{root}, Config{})
	if err != nil {
		t.Fatal(err)
	}
	assertPaths(t, got, []string{root})
}

func TestWalkNonRecursiveDoesNotReadDir(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "Dir")
	child := filepath.Join(root, "locked")
	mkdirs(t, root, "locked")
	if err := os.Chmod(child, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(child, 0o755) })

	got, err := walkRoot(root, Config{})
	if err != nil {
		t.Fatal(err)
	}
	assertPaths(t, got, []string{root})
}

func TestWalkRecursiveIncludesRootAndAllDepths(t *testing.T) {
	base := exampleTree(t)
	root := filepath.Join(base, "Test Dir")
	got, err := Walk([]string{root}, Config{Recursive: true})
	if err != nil {
		t.Fatal(err)
	}
	assertPaths(t, got, []string{
		root,
		filepath.Join(root, "Child One"),
		filepath.Join(root, "Child One", "file.txt"),
		filepath.Join(root, "Child Two"),
	})
}

func TestWalkFileArgument(t *testing.T) {
	base := t.TempDir()
	touch(t, base, "A.txt")
	f := filepath.Join(base, "A.txt")
	for _, rec := range []bool{false, true} {
		got, err := Walk([]string{f}, Config{Recursive: rec})
		if err != nil {
			t.Fatal(err)
		}
		assertPaths(t, got, []string{f})
	}
}

func TestWalkSkipsGitAndDSStore(t *testing.T) {
	base := t.TempDir()
	mkdirs(t, base, "Dir/.git/objects")
	touch(t, base, "Dir/.git/HEAD", "Dir/.DS_Store", "Dir/a.txt")
	root := filepath.Join(base, "Dir")

	// As root arguments.
	got, err := Walk([]string{
		filepath.Join(root, ".git"),
		filepath.Join(root, ".DS_Store"),
	}, Config{Recursive: true})
	if err != nil {
		t.Fatal(err)
	}
	assertPaths(t, got, nil)

	// During a recursive walk.
	got, err = Walk([]string{root}, Config{Recursive: true})
	if err != nil {
		t.Fatal(err)
	}
	assertPaths(t, got, []string{root, filepath.Join(root, "a.txt")})
}

func TestWalkTrailingSlash(t *testing.T) {
	base := exampleTree(t)
	chdir(t, base)
	for _, rec := range []bool{false, true} {
		cfg := Config{Recursive: rec}
		a, err := Walk([]string{"Test Dir/"}, cfg)
		if err != nil {
			t.Fatal(err)
		}
		b, err := Walk([]string{"Test Dir"}, cfg)
		if err != nil {
			t.Fatal(err)
		}
		assertPaths(t, a, b)
	}
}

func TestWalkDedupe(t *testing.T) {
	base := exampleTree(t)
	root := filepath.Join(base, "Test Dir")

	got, err := Walk([]string{root, root}, Config{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []string{root}) {
		t.Fatalf("got %v", got)
	}

	sub := filepath.Join(root, "Child One")
	got, err = Walk([]string{root, sub}, Config{Recursive: true})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, p := range got {
		if seen[p] {
			t.Errorf("duplicate entry %s", p)
		}
		seen[p] = true
	}
	if len(got) != 4 {
		t.Errorf("expected 4 entries, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// Rename (run)
// ---------------------------------------------------------------------------

func TestRunNonRecursiveRenamesOnlyDir(t *testing.T) {
	base := exampleTree(t)
	chdir(t, base)
	if err := run([]string{"Test Dir"}, Config{}); err != nil {
		t.Fatal(err)
	}
	exists(t, base, "test-dir/Child One/file.txt", "test-dir/Child Two")
	missing(t, base, "Test Dir")
}

func TestRunRecursiveExampleTree(t *testing.T) {
	base := exampleTree(t)
	chdir(t, base)
	if err := run([]string{"Test Dir"}, Config{Recursive: true}); err != nil {
		t.Fatal(err)
	}
	exists(t, base, "test-dir/child-one/file.txt", "test-dir/child-two")
	missing(t, base, "Test Dir", "test-dir/Child One", "test-dir/Child Two")
}

func TestRunDuplicateTargets(t *testing.T) {
	base := t.TempDir()
	touch(t, base, "A b", "a_b")
	chdir(t, base)
	if err := run([]string{"A b", "a_b"}, Config{}); err == nil {
		t.Fatal("expected collision error")
	}
	exists(t, base, "A b", "a_b")
	missing(t, base, "a-b")
}

func TestRunCaseOnlyDirRename(t *testing.T) {
	base := t.TempDir()
	mkdirs(t, base, "MyDir")
	touch(t, base, "MyDir/f.txt")
	chdir(t, base)
	if err := run([]string{"MyDir"}, Config{}); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(base)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "mydir" {
		t.Fatalf("unexpected entries: %v", entries)
	}
	exists(t, base, "mydir/f.txt")
}
