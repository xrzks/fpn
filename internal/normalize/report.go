package normalize

import (
	"fmt"
	"io"
	"strconv"
	"text/tabwriter"
	"unicode"
	"unicode/utf8"
)

// displayPath quotes paths that could inject control characters into output.
// Ordinary paths, including those containing spaces, are left unchanged.
func displayPath(path string) string {
	if path == "" || !utf8.ValidString(path) {
		return strconv.Quote(path)
	}
	for _, r := range path {
		if unicode.IsControl(r) {
			return strconv.Quote(path)
		}
	}
	return path
}

func logRenames(renames []renameOp, writer io.Writer) error {
	if len(renames) == 0 {
		return nil
	}
	w := tabwriter.NewWriter(writer, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(w, "ORIGINAL\tNORMALIZED"); err != nil {
		return err
	}
	for _, r := range renames {
		if _, err := fmt.Fprintf(w, "%s\t%s\n", displayPath(r.from), displayPath(r.to)); err != nil {
			return err
		}
	}
	return w.Flush()
}
