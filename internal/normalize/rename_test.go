//go:build linux || darwin || windows

package normalize

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestRenameNoReplaceDoesNotReplaceLateDestination(t *testing.T) {
	for _, kind := range []string{"file", "directory", "dangling symlink"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			from, to := filepath.Join(dir, "Source File"), filepath.Join(dir, "source-file")
			if kind == "directory" {
				if err := os.Mkdir(from, 0700); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(from, []byte("original"), 0600); err != nil {
				t.Fatal(err)
			}
			// Simulate a destination created after planning.
			switch kind {
			case "file":
				if err := os.WriteFile(to, []byte("competitor"), 0600); err != nil {
					t.Fatal(err)
				}
			case "directory":
				if err := os.Mkdir(to, 0700); err != nil {
					t.Fatal(err)
				}
			case "dangling symlink":
				if err := os.Symlink("missing", to); err != nil {
					t.Skipf("cannot create symlink: %v", err)
				}
			}
			before, err := os.Lstat(to)
			if err != nil {
				t.Fatal(err)
			}
			if err := renameNoReplace(from, to); !errors.Is(err, os.ErrExist) {
				t.Fatalf("expected destination-exists error, got %v", err)
			}
			after, err := os.Lstat(to)
			if err != nil || !os.SameFile(before, after) {
				t.Fatalf("destination replaced: %v", err)
			}
			if kind != "directory" {
				data, err := os.ReadFile(from)
				if err != nil || string(data) != "original" {
					t.Fatalf("source changed: %q, %v", data, err)
				}
			} else if info, err := os.Stat(from); err != nil || !info.IsDir() {
				t.Fatalf("source directory changed: %v", err)
			}
			entries, err := os.ReadDir(dir)
			if err != nil || len(entries) != 2 {
				t.Fatalf("unexpected entries: %v, %v", entries, err)
			}
		})
	}
}

func TestRenameNoReplaceConcurrent(t *testing.T) {
	const count = 16
	dir := t.TempDir()
	to := filepath.Join(dir, "destination")
	start := make(chan struct{})
	results := make(chan error, count)
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		f, err := os.CreateTemp(dir, "source-")
		if err != nil {
			t.Fatal(err)
		}
		from := f.Name()
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results <- renameNoReplace(from, to)
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	winners := 0
	for err := range results {
		if err == nil {
			winners++
		} else if !errors.Is(err, os.ErrExist) {
			t.Errorf("unexpected error: %v", err)
		}
	}
	if winners != 1 {
		t.Fatalf("got %d successful renames, want exactly one", winners)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != count {
		t.Fatalf("source entries lost: %v, %v", entries, err)
	}
}
