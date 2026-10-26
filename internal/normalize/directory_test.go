package normalize

import (
	"bytes"
	"errors"
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
	info, err := os.Lstat(filepath.Join(base, destination))
	if err != nil {
		t.Fatal(err)
	}
	actual, err := d.actualName("café", info)
	if err != nil || actual != destination {
		t.Fatalf("actualName = %q, %v; want distinct destination %q", actual, err, destination)
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
	info, err := os.Lstat(filepath.Join(base, "My File"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.actualName("MYFILE~1", info); err == nil {
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
	if err := Run([]string{"Café File", "Cafe\u0301_File"}, Config{Writer: &out}); err != nil {
		t.Fatal(err)
	}
	exists(t, base, "café-file", "cafe\u0301-file")
}
