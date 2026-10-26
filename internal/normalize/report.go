package normalize

import (
	"fmt"
	"io"
	"text/tabwriter"
)

func logRenames(renames []renameOp, writer io.Writer) {
	if len(renames) == 0 {
		return
	}
	w := tabwriter.NewWriter(writer, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ORIGINAL\tNORMALIZED")
	for _, r := range renames {
		fmt.Fprintf(w, "%s\t%s\n", r.from, r.to)
	}
	w.Flush()
}
