package normalize

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestConfigDefaultWriters(t *testing.T) {
	config := Config{}
	if config.writer() != os.Stdout || config.errWriter() != os.Stderr {
		t.Fatal("nil writers must default to standard output and standard error")
	}
}

func TestRunReportsToConfiguredWriters(t *testing.T) {
	for _, dry := range []bool{true, false} {
		t.Run(fmt.Sprintf("dry=%v", dry), func(t *testing.T) {
			base := t.TempDir()
			chdir(t, base)
			touch(t, base, "A b", "a_b", "Safe File")
			var out, diagnostics bytes.Buffer
			err := Run([]string{".git", "A b", "a_b", "Safe File"}, Config{
				DryRun: dry, Writer: &out, ErrWriter: &diagnostics,
			})
			if err == nil || !strings.Contains(err.Error(), "1 collision(s)") {
				t.Fatalf("expected returned collision summary, got %v", err)
			}
			wantDiagnostics := "fpn: .git: skipped\n" +
				"collision: a-b would be produced by multiple sources: A b, a_b\n"
			if diagnostics.String() != wantDiagnostics {
				t.Fatalf("diagnostics = %q, want %q", diagnostics.String(), wantDiagnostics)
			}
			wantTable := "ORIGINAL   NORMALIZED\nSafe File  safe-file\n"
			if out.String() != wantTable {
				t.Fatalf("table = %q, want %q", out.String(), wantTable)
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

func TestRunDoesNotPrintReturnedErrors(t *testing.T) {
	for _, args := range [][]string{nil, {"missing"}} {
		base := t.TempDir()
		chdir(t, base)
		var out, diagnostics bytes.Buffer
		err := Run(args, Config{Writer: &out, ErrWriter: &diagnostics})
		if err == nil {
			t.Fatalf("expected error for args %v", args)
		}
		if out.Len() != 0 || diagnostics.Len() != 0 {
			t.Fatalf("returned error was printed: stdout=%q, stderr=%q", out.String(), diagnostics.String())
		}
	}
}
