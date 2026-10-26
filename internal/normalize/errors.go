package normalize

import (
	"os"
	"strings"
)

// rootPathError reports the supplied path rather than repeating the resolved
// path and filesystem operation from an underlying PathError.
type rootPathError struct {
	path string
	err  error
}

func (e rootPathError) Unwrap() error { return e.err }
func (e rootPathError) Error() string {
	cause := e.err
	if pathErr, ok := cause.(*os.PathError); ok {
		cause = pathErr.Err
	}
	return "fpn: " + displayPath(e.path) + ": " + formatDiagnostic(cause)
}

// diagnosticError changes only rendering; unwrapping retains the original tree.
type diagnosticError struct{ error }

func (e diagnosticError) Unwrap() error { return e.error }
func (e diagnosticError) Error() string { return formatDiagnostic(e.error) }

func formatDiagnostic(err error) string {
	switch e := err.(type) {
	case rootPathError:
		return e.Error()
	case *os.PathError:
		return e.Op + " " + displayPath(e.Path) + ": " + formatDiagnostic(e.Err)
	case *os.LinkError:
		return e.Op + " " + displayPath(e.Old) + " " + displayPath(e.New) + ": " + formatDiagnostic(e.Err)
	case interface{ Unwrap() []error }:
		message := err.Error()
		for _, child := range e.Unwrap() {
			if child == nil {
				continue
			}
			before, after := child.Error(), formatDiagnostic(child)
			if before != after {
				if !strings.Contains(message, before) {
					return displayPath(err.Error())
				}
				message = strings.ReplaceAll(message, before, after)
			}
		}
		return message
	case interface{ Unwrap() error }:
		if child := e.Unwrap(); child != nil {
			before, after := child.Error(), formatDiagnostic(child)
			if before != after {
				if strings.HasSuffix(err.Error(), before) {
					return strings.TrimSuffix(err.Error(), before) + after
				}
				return displayPath(err.Error())
			}
		}
	}
	return err.Error()
}
