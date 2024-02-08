package main

import (
	"fmt"

	snsync "github.com/clayrosenthal/sn-sync/sn-sync"
	cli "github.com/urfave/cli/v2"
)

func diffCmd() *cli.Command {
	return &cli.Command{
		Name:  "diff",
		Usage: "display differences between local and remote",
		Action: func(c *cli.Context) error {
			opts, err := getOpts(c)
			if err != nil {
				return err
			}
			return diffCmdAction(c, opts)
		},
	}
}

func diffCmdAction(c *cli.Context, opts configOptsOutput) (err error) {
	if !opts.dotfiles && c.Args().Len() < 1 {
		_, _ = fmt.Fprintf(c.App.Writer, "error: specify path(s) to diff")
		_ = cli.ShowCommandHelp(c, "diff")
		return nil
	}

	session, err := getSession(opts)
	if err != nil {
		return err
	}

	root, paths := getRootAndPaths(c, opts)

	_, msg, err := snsync.Diff(&session, root, paths, opts.pageSize, true, c.Bool("no-stdout"))

	if err != nil {
		return err
	}

	if opts.display {
		_, _ = fmt.Fprintf(c.App.Writer, msg)
	}

	return err
}
