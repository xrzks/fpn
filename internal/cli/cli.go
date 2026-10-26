// Package cli defines the fpn command-line interface.
package cli

import (
	"context"

	"github.com/urfave/cli/v3"
	"github.com/xrzks/fpn/internal/normalize"
)

// New constructs an independent command using the standard streams.
func New() *cli.Command {
	return &cli.Command{
		Name:      "fpn",
		Usage:     "Normalize file and directory names",
		ArgsUsage: "<path>...",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "dry",
				Usage: "show what would happen without making changes",
			},
			&cli.BoolFlag{
				Name:    "recursive",
				Aliases: []string{"r"},
				Usage:   "also rename everything nested inside directory arguments",
			},
		},
		// Return errors to main without printing them twice or exiting here.
		OnUsageError: func(_ context.Context, _ *cli.Command, err error, _ bool) error {
			return err
		},
		Action: func(_ context.Context, cmd *cli.Command) error {
			return normalize.Run(cmd.Args().Slice(), normalize.Config{
				DryRun:    cmd.Bool("dry"),
				Recursive: cmd.Bool("recursive"),
				Writer:    cmd.Root().Writer,
				ErrWriter: cmd.Root().ErrWriter,
			})
		},
	}
}
