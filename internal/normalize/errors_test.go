package normalize

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunMissingPathDiagnostic(t *testing.T) {
	base := t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(base); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })

	for _, path := range []string{
		"ijferowijf",
		filepath.Join("missing-parent", "file"),
		filepath.Join(base, "missing"),
		"missing\nfile",
	} {
		for _, recursive := range []bool{false, true} {
			err := Run([]string{path}, Config{Recursive: recursive, Writer: io.Discard, ErrWriter: io.Discard})
			var pathErr *os.PathError
			if !errors.Is(err, os.ErrNotExist) || !errors.As(err, &pathErr) {
				t.Fatalf("path %q: lost missing-path cause: %v", path, err)
			}
			want := "fpn: " + displayPath(path) + ": " + pathErr.Err.Error()
			if got := err.Error(); got != want {
				t.Fatalf("Error() = %q, want %q", got, want)
			}
		}
	}
}

func TestRunEscapesReturnedPaths(t *testing.T) {
	for _, name := range []string{"missing\nfile", "missing\x1bfile", "missing\xfffile"} {
		t.Run(fmt.Sprintf("%q", name), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), name)
			err := Run([]string{path}, Config{Writer: io.Discard, ErrWriter: io.Discard})
			if err == nil {
				t.Fatal("expected missing-path error")
			}
			if got := err.Error(); strings.Contains(got, path) || !strings.Contains(got, displayPath(path)) {
				t.Fatalf("error does not safely render path: %q", got)
			}
			if !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("lost missing-file cause: %v", err)
			}
			var pathErr *os.PathError
			if !errors.As(err, &pathErr) || pathErr.Path != path {
				t.Fatalf("lost original PathError: %#v", pathErr)
			}
		})
	}
}

func TestDiagnosticErrorNestedPaths(t *testing.T) {
	cause := errors.New("failure")
	pathErr := &os.PathError{Op: "open", Path: "inner\npath", Err: cause}
	linkErr := &os.LinkError{Op: "rename", Old: "old\x1bpath", New: "new\npath", Err: pathErr}
	wrapped := fmt.Errorf("operation: %w", linkErr)
	original := errors.Join(wrapped, errors.New("ordinary\nmultiline diagnostic"))
	err := diagnosticError{original}
	want := "operation: rename " + displayPath(linkErr.Old) + " " + displayPath(linkErr.New) +
		": open " + displayPath(pathErr.Path) + ": failure\nordinary\nmultiline diagnostic"
	if got := err.Error(); got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}
	if err.Unwrap() != original || !errors.Is(err, cause) || !errors.Is(err, wrapped) {
		t.Fatal("lost original error tree")
	}
	var gotPath *os.PathError
	var gotLink *os.LinkError
	if !errors.As(err, &gotPath) || gotPath != pathErr || !errors.As(err, &gotLink) || gotLink != linkErr {
		t.Fatal("errors.As did not retain original path errors")
	}
}

func TestDiagnosticErrorOrdinaryMessagesUnchanged(t *testing.T) {
	plain := errors.New("ordinary\nmultiline diagnostic")
	pathErr := &os.PathError{Op: "open", Path: "ordinary path", Err: plain}
	for _, original := range []error{
		plain,
		pathErr,
		&os.LinkError{Op: "rename", Old: "old path", New: "new path", Err: plain},
		fmt.Errorf("context: %w", pathErr),
		errors.Join(plain, pathErr),
		fmt.Errorf("first: %w; second: %w", plain, pathErr),
	} {
		if got := (diagnosticError{original}).Error(); got != original.Error() {
			t.Fatalf("ordinary diagnostic changed: got %q, want %q", got, original.Error())
		}
	}
}

func TestDiagnosticErrorNonSuffixWrapper(t *testing.T) {
	pathErr := &os.PathError{Op: "open", Path: "unsafe\n\x1bpath", Err: os.ErrNotExist}
	original := fmt.Errorf("context: %w (details)", pathErr)
	if got, want := (diagnosticError{original}).Error(), displayPath(original.Error()); got != want {
		t.Fatalf("Error() = %q, want safely quoted fallback %q", got, want)
	}
}

func TestRunSuccessReturnsNil(t *testing.T) {
	if err := Run([]string{t.TempDir()}, Config{DryRun: true, Writer: io.Discard, ErrWriter: io.Discard}); err != nil {
		t.Fatalf("successful Run returned %v", err)
	}
}
