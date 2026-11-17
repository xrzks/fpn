// Package cli defines the fpn command-line interface.
package cli

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"

	"github.com/xrzks/fpn/internal/normalize"
)

// New builds the fpn command. version is reported by --version.
func New(version string) *cli.Command {
	return &cli.Command{
		Name:      "fpn",
		Usage:     "normalize file and directory names",
		Version:   version,
		ArgsUsage: "<path>",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "dry-run",
				Aliases: []string{"n"},
				Usage:   "show what would happen without making changes",
			},
			&cli.BoolFlag{
				Name:    "recursive",
				Aliases: []string{"r"},
				Usage:   "recursively rename files and symlinks, leaving all directories unchanged",
			},
		},
		// Return usage errors instead of printing them with the full help; main
		// prints the returned error, so this keeps it to a single line.
		OnUsageError: func(_ context.Context, _ *cli.Command, err error, _ bool) error {
			return fmt.Errorf("%w (see fpn --help)", err)
		},
		Action: func(_ context.Context, cmd *cli.Command) error {
			return normalize.Run(cmd.Args().Slice(), normalize.Config{
				DryRun:    cmd.Bool("dry-run"),
				Recursive: cmd.Bool("recursive"),
				Writer:    cmd.Root().Writer,
				ErrWriter: cmd.Root().ErrWriter,
			})
		},
	}
}
