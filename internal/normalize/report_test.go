package normalize

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDisplayPath(t *testing.T) {
	for _, tt := range []struct {
		path string
		want string
	}{
		{"ordinary", "ordinary"},
		{"Safe File", "Safe File"},
		{"café/文件", "café/文件"},
		{"", `""`},
		{"a\nb", `"a\nb"`},
		{"a\tb", `"a\tb"`},
		{"a\rb", `"a\rb"`},
		{"a\vb", `"a\vb"`},
		{"a\fb", `"a\fb"`},
		{"a\x1b[31mb", `"a\x1b[31mb"`},
		{"a\x00b", `"a\x00b"`},
		{"a\x7fb", `"a\x7fb"`},
		{"a\u0085b", `"a\u0085b"`},
		{"a\xffb", `"a\xffb"`}, // 0xff is also tabwriter.Escape.
		{"a\xc3b", `"a\xc3b"`},
	} {
		t.Run(fmt.Sprintf("%q", tt.path), func(t *testing.T) {
			if got := displayPath(tt.path); got != tt.want {
				t.Fatalf("displayPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
			var out bytes.Buffer
			if err := logRenames([]renameOp{{from: tt.path, to: tt.path}}, &out); err != nil {
				t.Fatal(err)
			}
			width := max(len("ORIGINAL"), len([]rune(tt.want))) + 2
			wantTable := fmt.Sprintf("%-*sNORMALIZED\n%-*s%s\n", width, "ORIGINAL", width, tt.want, tt.want)
			if out.String() != wantTable {
				t.Fatalf("table = %q, want %q", out.String(), wantTable)
			}
		})
	}
}

type failingReportWriter struct {
	err error
}

func (w failingReportWriter) Write([]byte) (int, error) {
	return 0, w.err
}

func TestLogRenamesWriterError(t *testing.T) {
	want := errors.New("report write failed")
	writer := failingReportWriter{err: want}
	if err := logRenames([]renameOp{{from: "Safe File", to: "safe-file"}}, writer); !errors.Is(err, want) {
		t.Fatalf("logRenames error = %v, want %v", err, want)
	}
	if err := logRenames(nil, writer); err != nil {
		t.Fatalf("empty report should not write: %v", err)
	}
}

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
			touch(t, base, ".git", "A b", "a_b", "Safe File")
			var out, diagnostics bytes.Buffer
			if err := Run([]string{".git"}, Config{Writer: &out, ErrWriter: &diagnostics}); err != nil {
				t.Fatal(err)
			}
			err := Run([]string{"."}, Config{
				Recursive: true, DryRun: dry, Writer: &out, ErrWriter: &diagnostics,
			})
			if err == nil || !strings.Contains(err.Error(), "1 collision(s)") {
				t.Fatalf("expected returned collision summary, got %v", err)
			}
			physical := physicalDir(t, base)
			wantDiagnostics := fmt.Sprintf("fpn: .git: skipped\ncollision: %s would be produced by multiple sources: %s, %s\n",
				filepath.Join(physical, "a-b"), filepath.Join(physical, "A b"), filepath.Join(physical, "a_b"))
			if diagnostics.String() != wantDiagnostics {
				t.Fatalf("diagnostics = %q, want %q", diagnostics.String(), wantDiagnostics)
			}
			if out.Len() != 0 {
				t.Fatalf("failed plan must not report proposed renames: %q", out.String())
			}
			exists(t, base, "Safe File", "A b", "a_b")
			missing(t, base, "safe-file", "a-b")
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
