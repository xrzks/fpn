package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLongRenamePath(t *testing.T) {
	longPart := strings.Repeat("a", 250)
	for _, tt := range []struct{ path, want string }{
		{`C:\short`, `C:\short`},
		{`C:\` + longPart, `\\?\C:\` + longPart},
		{`\\server\share\` + longPart, `\\?\UNC\server\share\` + longPart},
		{`\\?\C:\` + longPart, `\\?\C:\` + longPart},
		{`\\.\C:\` + longPart, `\\.\C:\` + longPart},
	} {
		got, err := longRenamePath(tt.path)
		if err != nil || got != tt.want {
			t.Errorf("longRenamePath(%q) = %q, %v; want %q", tt.path, got, err, tt.want)
		}
	}
}

func TestRenameOneLongWindowsPath(t *testing.T) {
	dir := filepath.Join(t.TempDir(), strings.Repeat("a", 100), strings.Repeat("b", 100), strings.Repeat("c", 100))
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	from := filepath.Join(dir, "SOURCE")
	for _, caseOnly := range []bool{false, true} {
		to := filepath.Join(dir, "destination")
		if caseOnly {
			to = filepath.Join(dir, "source")
		}
		if err := os.WriteFile(from, []byte("original"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := renameOne(renameOp{from: from, to: to, caseOnly: caseOnly}); err != nil {
			t.Fatal(err)
		}
		if data, err := os.ReadFile(to); err != nil || string(data) != "original" {
			t.Fatalf("source not preserved: %q, %v", data, err)
		}
		if err := os.Remove(to); err != nil {
			t.Fatal(err)
		}
	}
}
