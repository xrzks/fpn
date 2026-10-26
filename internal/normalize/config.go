package normalize

import (
	"errors"
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

// Run walks and normalizes the supplied paths. The caller is responsible for
// reporting the returned error.
func Run(args []string, config Config) error {
	var argsErr error
	if len(args) < 1 {
		argsErr = fmt.Errorf("please specify one or more directories or files")
	}
	paths, walkErr := Walk(args, config)
	normErr := NormalizePaths(paths, config)
	return errors.Join(argsErr, walkErr, normErr)
}
