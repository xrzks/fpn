package normalize

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/text/unicode/norm"
)

func TestFailedPlanLeavesSafeEntriesUntouched(t *testing.T) {
	for _, dry := range []bool{false, true} {
		for _, kind := range []string{"existing destination", "multiple sources", "invalid basename"} {
			t.Run(fmt.Sprintf("%s/dry=%v", kind, dry), func(t *testing.T) {
				base := t.TempDir()
				touch(t, base, "A Safe File")
				switch kind {
				case "existing destination":
					touch(t, base, "Z File", "z-file")
				case "multiple sources":
					touch(t, base, "Z File", "z_file")
				case "invalid basename":
					touch(t, base, " . ")
				}
				var out bytes.Buffer
				err := Run([]string{base}, Config{Recursive: true, DryRun: dry, Writer: &out, ErrWriter: io.Discard})
				if err == nil || out.Len() != 0 {
					t.Fatalf("failed plan: error=%v, output=%q", err, &out)
				}
				exists(t, base, "A Safe File")
				missing(t, base, "a-safe-file")
			})
		}
	}
}

func TestNativeSourceAliasesRejected(t *testing.T) {
	for _, directory := range []bool{false, true} {
		for _, unicodeAlias := range []bool{false, true} {
			t.Run(fmt.Sprintf("directory=%v/unicode=%v", directory, unicodeAlias), func(t *testing.T) {
				base := t.TempDir()
				chdir(t, base)
				name := "Source File"
				if unicodeAlias {
					name = "Café File"
				}
				if directory {
					mkdirs(t, base, name)
					touch(t, filepath.Join(base, name), "Child File")
				} else {
					touch(t, base, name)
				}
				// Some filesystems store a different Unicode normalization form.
				before := planningEntryNames(t)
				stored := before[0]
				alternate := strings.ToLower(stored)
				if unicodeAlias {
					alternate = norm.NFD.String(stored)
					if alternate == stored {
						alternate = norm.NFC.String(stored)
					}
				}
				requirePlanningAlias(t, stored, alternate)
				for _, dry := range []bool{false, true} {
					var out bytes.Buffer
					err := Run([]string{alternate}, Config{Recursive: directory, DryRun: dry, Writer: &out, ErrWriter: io.Discard})
					if err == nil || !strings.Contains(err.Error(), "does not exactly match") || out.Len() != 0 {
						t.Fatalf("source alias accepted: error=%v, output=%q", err, &out)
					}
					assertPaths(t, planningEntryNames(t), before)
					if directory {
						exists(t, base, filepath.Join(stored, "Child File"))
						missing(t, base, filepath.Join(stored, "child-file"))
					}
				}
			})
		}
	}
}

func TestRunReportsPhysicalPathsThroughDotDot(t *testing.T) {
	base := t.TempDir()
	mkdirs(t, base, "local", "physical/child")
	touch(t, base, "local/My File", "physical/My File")
	walkSymlink(t, filepath.Join(base, "physical/child"), filepath.Join(base, "local/link"))
	var out bytes.Buffer
	root := filepath.Join(base, "local/link") + "/../My File"
	if err := Run([]string{root}, Config{Writer: &out}); err != nil {
		t.Fatal(err)
	}
	physical := physicalDir(t, filepath.Join(base, "physical"))
	for _, name := range []string{"My File", "my-file"} {
		if !strings.Contains(out.String(), filepath.Join(physical, name)) {
			t.Fatalf("missing physical path for %q: %q", name, &out)
		}
	}
	if data, err := os.ReadFile(filepath.Join(base, "local/My File")); err != nil || len(data) != 0 {
		t.Fatalf("unselected local entry changed: %q, %v", data, err)
	}
}
