package main

import (
	"fmt"

	snsync "github.com/clayrosenthal/sn-sync/sn-sync"
	cli "github.com/urfave/cli/v2"
)

func syncCmd() *cli.Command {
	return &cli.Command{
		Name:    "dir",
		Aliases: []string{"directory", "d"},
		Usage:   "sync directory",
		Flags: []cli.Flag{
			&cli.StringSliceFlag{
				Name:  "exclude",
				Usage: "exlude path from sync",
			},
		},
		BashComplete: func(c *cli.Context) {
			syncTasks := []string{"--exclude"}
			for _, t := range syncTasks {
				fmt.Println(t)
			}
		},
		Action: func(c *cli.Context) error {
			return syncCmdFunc(c, false)
		},
	}
}

func syncCmdFunc(c *cli.Context, dotfiles bool) (err error) {
	var opts configOptsOutput
	opts, err = getOpts(c)
	if err != nil {
		return err
	}

	if c.Args().Len() < 1 {
		_, _ = fmt.Fprintf(c.App.Writer, "error: specify path(s) to sync")
		_ = cli.ShowCommandHelp(c, "sync")
		return nil
	}

	session, err := getSession(opts)
	if err != nil {
		return err
	}

	root, paths := getRootAndPaths(c, opts)

	var so snsync.SyncOutput
	so, err = snsync.Sync(snsync.SNDirSyncInput{
		Session:  &session,
		Root:     root,
		Paths:    paths,
		Exclude:  c.StringSlice("exclude"),
		PageSize: opts.pageSize,
		Debug:    opts.debug,
	}, c.Bool("no-stdout"))

	if err != nil {
		return err
	}

	if opts.display {
		_, _ = fmt.Fprintf(c.App.Writer, so.Msg)
	}

	return err
}
