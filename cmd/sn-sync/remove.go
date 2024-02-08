package main

import (
	"fmt"

	snsync "github.com/clayrosenthal/sn-sync/sn-sync"
	cli "github.com/urfave/cli/v2"
)

func removeCmd() *cli.Command {
	return &cli.Command{
		Name:    "remove",
		Aliases: []string{"rm"},
		Usage:   "stop tracking file(s)",
		Action: func(c *cli.Context) error {
			opts, err := getOpts(c)
			if err != nil {
				return err
			}
			return removeCmdAction(c, opts)
		},
	}
}

func removeCmdAction(c *cli.Context, opts configOptsOutput) (err error) {
	if c.Args().Len() == 0 {
		_, _ = fmt.Fprintf(c.App.Writer, "error: paths not specified")
		_ = cli.ShowCommandHelp(c, "remove")
		return nil
	}

	session, err := getSession(opts)
	if err != nil {
		return err
	}

	root, paths := getRootAndPaths(c, opts)

	ri := snsync.RemoveInput{
		Session:  &session,
		Root:     root,
		Paths:    paths,
		PageSize: opts.pageSize,
		Debug:    opts.debug,
	}

	var ro snsync.RemoveOutput

	ro, err = snsync.Remove(ri, c.Bool("no-stdout"))
	if err != nil {
		return err
	}
	if opts.display {
		_, _ = fmt.Fprintf(c.App.Writer, ro.Msg)
	}

	return err
}
