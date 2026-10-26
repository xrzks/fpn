package normalize

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
)

func TestRunRecursiveDotPreservesRoot(t *testing.T) {
	base := t.TempDir()
	mkdirs(t, base, "Root Dir")
	root := filepath.Join(base, "Root Dir")
	touch(t, root, "Child File")
	chdir(t, root)
	var out bytes.Buffer
	if err := Run([]string{"."}, Config{Recursive: true, Writer: &out}); err != nil {
		t.Fatal(err)
	}
	exists(t, base, "Root Dir/child-file")
	missing(t, base, "root-dir", "Root Dir/Child File")
	if got, want := out.String(), "ORIGINAL    NORMALIZED\nChild File  child-file\n"; got != want {
		t.Fatalf("report = %q, want %q", got, want)
	}
}

func TestRunReportsOnlyCompletedRenamesOnFailure(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("requires enforced Unix directory permissions and a non-root user")
	}
	base := t.TempDir()
	chdir(t, base)
	mkdirs(t, base, "good", "locked")
	touch(t, base, "good/A File", "locked/B File")
	if err := os.Chmod("locked", 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(base, "locked"), 0o755) })
	if err := os.Rename("locked/B File", "locked/probe"); err == nil {
		if err := os.Rename("locked/probe", "locked/B File"); err != nil {
			t.Fatal(err)
		}
		t.Skip("directory permissions do not prevent renames")
	} else if !errors.Is(err, os.ErrPermission) {
		t.Fatalf("permission probe: %v", err)
	}
	var out bytes.Buffer
	// Execution sorts physical paths, so good completes even with reversed args.
	err := Run([]string{"locked/B File", "good/A File"}, Config{Writer: &out})
	if !errors.Is(err, os.ErrPermission) {
		t.Fatalf("Run error = %v, want permission failure", err)
	}
	exists(t, base, "good/a-file", "locked/B File")
	missing(t, base, "good/A File", "locked/b-file")
	from, to := filepath.Join("good", "A File"), filepath.Join("good", "a-file")
	want := fmt.Sprintf("%-*sNORMALIZED\n%s  %s\n", len(from)+2, "ORIGINAL", from, to)
	if out.String() != want {
		t.Fatalf("report = %q, want only completed rename %q", out.String(), want)
	}
}

func TestRunPropagatesReportWriterFailure(t *testing.T) {
	for _, dry := range []bool{true, false} {
		t.Run(fmt.Sprintf("dry=%v", dry), func(t *testing.T) {
			base := t.TempDir()
			chdir(t, base)
			touch(t, base, "Safe File")
			want := errors.New("report unavailable")
			err := Run([]string{"Safe File"}, Config{DryRun: dry, Writer: failingReportWriter{err: want}})
			if !errors.Is(err, want) {
				t.Fatalf("Run error = %v, want %v", err, want)
			}
			if dry {
				exists(t, base, "Safe File")
				missing(t, base, "safe-file")
			} else {
				exists(t, base, "safe-file")
				missing(t, base, "Safe File")
			}
		})
	}
}

func TestRunCollisionDiagnosticFailurePreventsMutations(t *testing.T) {
	base := t.TempDir()
	chdir(t, base)
	touch(t, base, "A File", "a_file", "Safe File")
	want := errors.New("diagnostics unavailable")
	var out bytes.Buffer
	err := Run([]string{"Safe File", "A File", "a_file"}, Config{
		Writer: &out, ErrWriter: failingReportWriter{err: want},
	})
	if !errors.Is(err, want) {
		t.Fatalf("Run error = %v, want %v", err, want)
	}
	exists(t, base, "A File", "a_file", "Safe File")
	missing(t, base, "a-file", "safe-file")
	if out.Len() != 0 {
		t.Fatalf("reported uncompleted renames: %q", out.String())
	}
}

func TestRunPropagatesSkipDiagnosticFailure(t *testing.T) {
	base := t.TempDir()
	chdir(t, base)
	touch(t, base, ".git")
	want := errors.New("skip diagnostic unavailable")
	err := Run([]string{".git"}, Config{Writer: io.Discard, ErrWriter: failingReportWriter{err: want}})
	if !errors.Is(err, want) {
		t.Fatalf("Run error = %v, want %v", err, want)
	}
	exists(t, base, ".git")
}

func TestRunDestinationNameTooLongIsNotCollision(t *testing.T) {
	base := t.TempDir()
	chdir(t, base)
	// U+023A lowercases to U+2C65: each two-byte rune becomes three bytes.
	source := "B" + strings.Repeat("\u023a", 100)
	target := "b" + strings.Repeat("\u2c65", 100)
	touch(t, base, source)
	_, probeErr := os.Lstat(target)
	if runtime.GOOS == "windows" && !errors.Is(probeErr, syscall.ENAMETOOLONG) {
		t.Skip("platform allows this destination length or uses a different length error")
	}
	if !errors.Is(probeErr, syscall.ENAMETOOLONG) {
		t.Fatalf("destination length probe = %v, want ENAMETOOLONG", probeErr)
	}
	var out, diagnostics bytes.Buffer
	err := Run([]string{source}, Config{Writer: &out, ErrWriter: &diagnostics})
	if !errors.Is(err, syscall.ENAMETOOLONG) {
		t.Fatalf("Run error = %v, want underlying ENAMETOOLONG", err)
	}
	if strings.Contains(err.Error(), "collision") || diagnostics.Len() != 0 || out.Len() != 0 {
		t.Fatalf("destination inspection error treated as collision: error=%v, stdout=%q, stderr=%q", err, out.String(), diagnostics.String())
	}
	exists(t, base, source)
}

// requirePlanningAlias verifies a native lookup alias, not merely two hardlinks.
func requirePlanningAlias(t *testing.T, stored, alternate string) {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() == alternate {
			t.Skipf("alternate spelling %q is a separately stored entry", alternate)
		}
	}
	original, err := os.Lstat(stored)
	if err != nil {
		t.Fatal(err)
	}
	alias, err := os.Lstat(alternate)
	if os.IsNotExist(err) {
		t.Skipf("filesystem does not resolve %q as %q", alternate, stored)
	}
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(original, alias) {
		t.Skip("alternate spelling does not resolve to the same entry")
	}
}

func planningEntryNames(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

func TestRunNativeCaseAliases(t *testing.T) {
	for _, tt := range []struct {
		name, stored string
		args         []string
		want         string
		report       string
	}{
		{"lower argument normalizes stored uppercase", "LICENSE", []string{"license"}, "license", "ORIGINAL  NORMALIZED\nLICENSE   license\n"},
		{"upper argument preserves normalized entry", "license", []string{"LICENSE"}, "license", ""},
		{"duplicate aliases rename once", "MyFile", []string{"MyFile", "MYFILE"}, "myfile", "ORIGINAL  NORMALIZED\nMyFile    myfile\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			base := t.TempDir()
			chdir(t, base)
			touch(t, base, tt.stored)
			alternate := strings.ToUpper(tt.stored)
			if alternate == tt.stored {
				alternate = strings.ToLower(tt.stored)
			}
			requirePlanningAlias(t, tt.stored, alternate)
			var out bytes.Buffer
			if err := Run(tt.args, Config{Writer: &out}); err != nil {
				t.Fatal(err)
			}
			assertPaths(t, planningEntryNames(t), []string{tt.want})
			if out.String() != tt.report {
				t.Fatalf("report = %q, want %q", out.String(), tt.report)
			}
		})
	}
}

func TestRunCanonicalEquivalentTargetsBothRemainUnchanged(t *testing.T) {
	base := t.TempDir()
	chdir(t, base)
	composed, decomposed := "Caf\u00e9 File", "Cafe\u0301_File"
	touch(t, base, composed, decomposed)
	requirePlanningAlias(t, composed, "Cafe\u0301 File")
	before := planningEntryNames(t)
	var out, diagnostics bytes.Buffer
	err := Run([]string{composed, decomposed}, Config{Writer: &out, ErrWriter: &diagnostics})
	if err == nil || !strings.Contains(err.Error(), "1 collision(s)") {
		t.Fatalf("Run error = %v, want canonical-equivalence collision", err)
	}
	assertPaths(t, planningEntryNames(t), before)
	exists(t, base, composed, decomposed)
	if out.Len() != 0 || !strings.Contains(diagnostics.String(), "multiple sources") {
		t.Fatalf("stdout=%q, stderr=%q; want no renames and a multiple-source collision", out.String(), diagnostics.String())
	}
}

func TestRunCanonicalEquivalentHardlinkIsCollision(t *testing.T) {
	base := t.TempDir()
	chdir(t, base)
	source, destination := "Caf\u00e9 File", "cafe\u0301-file"
	touch(t, base, source, "Safe File")
	requirePlanningAlias(t, source, "Cafe\u0301 File")
	if err := os.Link(source, destination); err != nil {
		t.Skipf("hardlinks unavailable: %v", err)
	}
	requirePlanningAlias(t, destination, "caf\u00e9-file")
	before, err := os.Lstat(source)
	if err != nil {
		t.Fatal(err)
	}
	var out, diagnostics bytes.Buffer
	err = Run([]string{source, "Safe File"}, Config{Writer: &out, ErrWriter: &diagnostics})
	if err == nil || !strings.Contains(err.Error(), "1 collision(s)") {
		t.Fatalf("Run error = %v, want existing hardlink collision", err)
	}
	exists(t, base, source, destination, "safe-file")
	missing(t, base, "Safe File")
	for _, name := range []string{source, destination} {
		after, err := os.Lstat(name)
		if err != nil || !os.SameFile(before, after) {
			t.Fatalf("entry %q changed: info=%v, error=%v", name, after, err)
		}
	}
	if got, want := out.String(), "ORIGINAL   NORMALIZED\nSafe File  safe-file\n"; got != want {
		t.Fatalf("report = %q, want %q", got, want)
	}
	if !strings.Contains(diagnostics.String(), "would overwrite existing file") {
		t.Fatalf("diagnostics = %q, want existing-entry collision", diagnostics.String())
	}
}
