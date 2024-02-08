package main

import (
	"fmt"

	snsync "github.com/clayrosenthal/sn-sync/sn-sync"
	cli "github.com/urfave/cli/v2"
)

func wipeCmd() *cli.Command {
	return &cli.Command{
		Name:  "wipe",
		Usage: "remove all sync",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "force",
				Usage: "assume user confirmation",
			},
		},
		BashComplete: func(c *cli.Context) {
			tasks := []string{"--force"}
			if c.NArg() > 0 {
				return
			}
			for _, t := range tasks {
				fmt.Println(t)
			}
		},
		Hidden: true,
		Action: func(c *cli.Context) error {
			opts, err := getOpts(c)
			if err != nil {
				return err
			}
			return wipeCmdAction(c, opts)
		},
	}
}

func wipeCmdAction(c *cli.Context, opts configOptsOutput) (err error) {
	if !opts.dotfiles && c.Args().Len() < 1 {
		_, _ = fmt.Fprintf(c.App.Writer, "error: specify path(s) to wipe")
		_ = cli.ShowCommandHelp(c, "wipe")
		return nil
	}

	var email string
	session, err := getSession(opts)
	if err != nil {
		return err
	}

	root, _ := getRootAndPaths(c, opts)

	var proceed bool
	if c.Bool("force") {
		proceed = true
	} else {
		_, _ = fmt.Fprintf(c.App.Writer, "wipe all sync for account %s? ", email)
		var input string
		_, err = fmt.Scanln(&input)
		if err == nil && snsync.StringInSlice(input, []string{"y", "yes"}, false) {
			proceed = true
		}
	}
	if proceed {
		var num int
		num, err = snsync.WipeDirTagsAndNotes(&session, root, opts.pageSize, c.Bool("no-stdout"))
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(c.App.Writer, "%d removed", num)
	} else {
		return nil
	}

	return err
}
