package main

import (
	"fmt"

	snsync "github.com/clayrosenthal/sn-sync/sn-sync"
	cli "github.com/urfave/cli/v2"
)

func statusCmd() *cli.Command {
	return &cli.Command{
		Name:    "status",
		Aliases: []string{"stat"},
		Usage:   "compare local and remote",
		Action: func(c *cli.Context) error {
			opts, err := getOpts(c)
			if err != nil {
				return err
			}
			return statusCmdAction(c, opts)
		},
	}
}

func statusCmdAction(c *cli.Context, opts configOptsOutput) (err error) {
	session, err := getSession(opts)
	if err != nil {
		return err
	}

	root, paths := getRootAndPaths(c, opts)

	_, msg, err := snsync.Status(&session, root, paths, opts.pageSize, opts.debug, false)
	if err != nil {
		return err
	}

	if opts.display {
		_, _ = fmt.Fprintf(c.App.Writer, msg)
	}
	return err
}
