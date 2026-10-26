package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHelp(t *testing.T) {
	for _, flag := range []string{"--help", "-h"} {
		t.Run(flag, func(t *testing.T) {
			var out, diagnostics bytes.Buffer
			if err := New(&out, &diagnostics).Run(context.Background(), []string{"fpn", flag}); err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{"fpn", "<path>...", "--dry", "--recursive", "-r", "--help"} {
				if !strings.Contains(out.String(), want) {
					t.Errorf("help missing %q: %s", want, &out)
				}
			}
			if diagnostics.Len() != 0 {
				t.Fatalf("unexpected diagnostics: %s", &diagnostics)
			}
		})
	}
}

func TestInvalidArguments(t *testing.T) {
	for _, args := range [][]string{nil, {"--dirs"}, {"--unknown"}, {"--dry=invalid"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var out, diagnostics bytes.Buffer
			err := New(&out, &diagnostics).Run(context.Background(), append([]string{"fpn"}, args...))
			if err == nil {
				t.Fatal("expected error")
			}
			if len(args) == 0 && !strings.Contains(err.Error(), "please specify") {
				t.Fatalf("unexpected error: %v", err)
			}
			if out.Len() != 0 || diagnostics.Len() != 0 {
				t.Fatalf("returned error should be reported only by main: stdout=%q stderr=%q", &out, &diagnostics)
			}
		})
	}
}

func TestRenameFlags(t *testing.T) {
	for _, recursive := range []string{"", "--recursive", "-r"} {
		for _, dry := range []bool{false, true} {
			t.Run(recursive+map[bool]string{true: "/dry", false: "/rename"}[dry], func(t *testing.T) {
				base := t.TempDir()
				root := filepath.Join(base, "Test Dir")
				if err := os.Mkdir(root, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, "Child File"), []byte("content"), 0o644); err != nil {
					t.Fatal(err)
				}
				args := []string{"fpn"}
				if recursive != "" {
					args = append(args, recursive)
				}
				if dry {
					args = append(args, "--dry")
				}
				args = append(args, root)
				var out, diagnostics bytes.Buffer
				if err := New(&out, &diagnostics).Run(context.Background(), args); err != nil {
					t.Fatal(err)
				}
				wantRoot, wantChild := "test-dir", "Child File"
				if dry {
					wantRoot = "Test Dir"
				} else if recursive != "" {
					wantChild = "child-file"
				}
				content, err := os.ReadFile(filepath.Join(base, wantRoot, wantChild))
				if err != nil || string(content) != "content" {
					t.Fatalf("content=%q error=%v", content, err)
				}
				if !strings.Contains(out.String(), "ORIGINAL") || !strings.Contains(out.String(), "test-dir") {
					t.Fatalf("missing rename table: %s", &out)
				}
				if diagnostics.Len() != 0 {
					t.Fatalf("unexpected diagnostics: %s", &diagnostics)
				}
			})
		}
	}
}

func TestEndOfFlags(t *testing.T) {
	base := t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(base); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
	if err := os.WriteFile("-My File", nil, 0o644); err != nil {
		t.Fatal(err)
	}
	var out, diagnostics bytes.Buffer
	if err := New(&out, &diagnostics).Run(context.Background(), []string{"fpn", "--", "-My File"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat("-my-file"); err != nil {
		t.Fatal(err)
	}
}
