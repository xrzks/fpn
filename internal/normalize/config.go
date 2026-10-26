package normalize

import (
	"fmt"
	"io"
	"os"
)

// Config controls traversal, renaming, and diagnostic output.
type Config struct {
	DryRun    bool
	Recursive bool
	// Writer receives rename tables. A nil Writer defaults to os.Stdout.
	Writer io.Writer
	// ErrWriter receives skip and collision diagnostics. A nil ErrWriter
	// defaults to os.Stderr. Returned errors are not printed by the engine.
	ErrWriter io.Writer
}

func (c Config) writer() io.Writer {
	if c.Writer != nil {
		return c.Writer
	}
	return os.Stdout
}

func (c Config) errWriter() io.Writer {
	if c.ErrWriter != nil {
		return c.ErrWriter
	}
	return os.Stderr
}

// Run walks and normalizes exactly one supplied path. The caller is responsible
// for reporting the returned error. Invalid input and walk failures never rename
// any entries.
func Run(args []string, config Config) error {
	if len(args) != 1 {
		return diagnosticError{fmt.Errorf("please specify exactly one directory or file")}
	}
	if args[0] == "" {
		return diagnosticError{fmt.Errorf("fpn: path argument must not be empty")}
	}
	paths, err := Walk(args[0], config)
	if err != nil {
		return diagnosticError{err}
	}
	if err := NormalizePaths(paths, config); err != nil {
		return diagnosticError{err}
	}
	return nil
}
