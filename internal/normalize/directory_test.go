package normalize

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestUnrelatedLongProbeDoesNotBlockRename(t *testing.T) {
	base := t.TempDir()
	chdir(t, base)
	// The stored name fits; its optional decomposed alias probe may not.
	long := "0" + strings.Repeat("é", 100)
	touch(t, base, long, "Safe File")
	if err := Run([]string{"Safe File"}, Config{Writer: io.Discard}); err != nil {
		t.Fatal(err)
	}
	exists(t, base, long, "safe-file")
}

func TestDirectoryKeysRespectLookupRules(t *testing.T) {
	d := &directory{}
	composed, decomposed := "café", "cafe\u0301"
	if d.nameKey(composed) == d.nameKey(decomposed) {
		t.Fatal("byte-sensitive lookup must keep canonical spellings distinct")
	}
	d.ignoreEncoding = true
	if d.nameKey(composed) != d.nameKey(decomposed) {
		t.Fatal("canonical aliases must share a target key")
	}
	if d.nameKey("CAFÉ") == d.nameKey(composed) {
		t.Fatal("case-sensitive lookup must keep case distinct")
	}
	d.ignoreCase = true
	if d.nameKey("CAFÉ") != d.nameKey(decomposed) || d.nameKey("σ") != d.nameKey("ς") {
		t.Fatal("casefold aliases must share target keys")
	}
	if d.nameKey("ß") == d.nameKey("ss") {
		t.Fatal("simple case folding must preserve expansion-distinct names")
	}
	d.fullCaseFold = true
	if d.nameKey("ß") != d.nameKey("ss") {
		t.Fatal("full casefold aliases must share target keys")
	}
}

func TestUnicodeTargetCollisionsPreventAllRenames(t *testing.T) {
	for _, tt := range []struct {
		name, probe, first, second, alias string
	}{
		{"negative initial case probe", "0ı", "ſ File", "s_File", "s File"},
		{"expanding case fold", "", "ß File", "ss_File", "ss File"},
	} {
		for _, dry := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/dry=%v", tt.name, dry), func(t *testing.T) {
				base := t.TempDir()
				chdir(t, base)
				touch(t, base, "A File", tt.first, tt.second)
				if tt.probe != "" {
					touch(t, base, tt.probe)
				}
				requirePlanningAlias(t, tt.first, tt.alias)
				before := planningEntryNames(t)
				var out, diagnostics bytes.Buffer
				err := Run([]string{"."}, Config{Recursive: true, DryRun: dry, Writer: &out, ErrWriter: &diagnostics})
				if err == nil || !strings.Contains(diagnostics.String(), "multiple sources") {
					t.Fatalf("expected preflight collision: error=%v, diagnostics=%q", err, &diagnostics)
				}
				if out.Len() != 0 {
					t.Fatalf("failed plan reported renames: %q", &out)
				}
				assertPaths(t, planningEntryNames(t), before)
			})
		}
	}
}

func TestExpansionDistinctTargetsRemainIndependent(t *testing.T) {
	base := t.TempDir()
	chdir(t, base)
	touch(t, base, "A File", "ß File", "ss_File")
	if _, err := os.Lstat("ss File"); err == nil {
		t.Skip("filesystem aliases expanded case folds")
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if err := Run([]string{"."}, Config{Recursive: true, Writer: io.Discard}); err != nil {
		t.Fatal(err)
	}
	assertPaths(t, planningEntryNames(t), []string{"a-file", "ss-file", "ß-file"})
}

func TestCanonicalAliasDoesNotSelectWrongCaseHardlink(t *testing.T) {
	base := t.TempDir()
	source, destination := "CAFÉ", "cafe\u0301"
	touch(t, base, source)
	if err := os.Link(filepath.Join(base, source), filepath.Join(base, destination)); err != nil {
		t.Skipf("hardlinks unavailable: %v", err)
	}
	var cache directoryCache
	d, err := cache.get(base)
	if err != nil {
		t.Fatal(err)
	}
	// Model case-sensitive, normalization-insensitive lookup, even on Linux.
	d.ignoreCase, d.ignoreEncoding = false, true
	if err := d.validateSource("café"); err == nil {
		t.Fatal("canonical alias must not select either stored hardlink")
	}
	for _, name := range []string{source, destination} {
		if err := d.validateSource(name); err != nil {
			t.Fatalf("exact source %q rejected: %v", name, err)
		}
	}
}

func TestProbeTooLongIsInconclusive(t *testing.T) {
	base := t.TempDir()
	touch(t, base, "File")
	var cache directoryCache
	d, err := cache.get(base)
	if err != nil {
		t.Fatal(err)
	}
	alternate := strings.Repeat("x", 300)
	if _, err := os.Lstat(filepath.Join(base, alternate)); !errors.Is(err, syscall.ENAMETOOLONG) {
		t.Skip("filesystem permits this component length or uses another length error")
	}
	alias, checked, err := d.isAlias("File", alternate)
	if alias || checked || err != nil {
		t.Fatalf("oversize optional probe = %v, %v, %v", alias, checked, err)
	}
}

func TestUnknownAliasFailsWithoutScanningHardlinks(t *testing.T) {
	base := t.TempDir()
	touch(t, base, "My File")
	var cache directoryCache
	d, err := cache.get(base)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.validateSource("MYFILE~1"); err == nil {
		t.Fatal("unknown short-name alias must fail closed rather than guess an entry")
	}
}

func TestCaseDistinctUnicodeTargetsRemainIndependent(t *testing.T) {
	base := t.TempDir()
	chdir(t, base)
	touch(t, base, "Café File", "Cafe\u0301_File")
	if _, err := os.Lstat("Cafe\u0301 File"); err == nil {
		t.Skip("normalization-insensitive filesystem")
	}
	var out bytes.Buffer
	if err := Run([]string{"."}, Config{Recursive: true, Writer: &out}); err != nil {
		t.Fatal(err)
	}
	exists(t, base, "café-file", "cafe\u0301-file")
}
