package main

import (
	"fmt"
	"path/filepath"

	snsync "github.com/clayrosenthal/sn-sync/sn-sync"
	cli "github.com/urfave/cli/v2"
)

func addCmd() *cli.Command {
	return &cli.Command{
		Name:  "add",
		Usage: "upload file(s)",
		Action: func(c *cli.Context) (err error) {
			opts, err := getOpts(c)
			if err != nil {
				return err
			}
			return addCmdAction(c, opts)
		},
	}
}

func addCmdAction(c *cli.Context, opts configOptsOutput) (err error) {
	if opts.dotfiles {
		if !c.Bool("all") && c.Args().Len() == 0 {
			_, _ = fmt.Fprintf(c.App.Writer, "error: either specify paths to add or --all to add everything")
			_ = cli.ShowCommandHelp(c, "add")
			return nil
		}

		if c.Bool("all") && c.Args().Len() > 0 {
			_, _ = fmt.Fprintf(c.App.Writer, "error: specifying --all and paths does not make sense")
			_ = cli.ShowCommandHelp(c, "add")
			return nil
		}
	} else {
		if c.Args().Len() < 1 {
			_, _ = fmt.Fprintf(c.App.Writer, "error: specify path(s) to add")
			_ = cli.ShowCommandHelp(c, "add")
			return nil
		}
	}

	session, err := getSession(opts)
	if err != nil {
		return err
	}

	root, paths := getRootAndPaths(c, opts)

	var absPaths []string
	for _, path := range paths {
		var ap string
		ap, err = filepath.Abs(path)
		if err != nil {
			return err
		}
		if opts.dotfiles && !isValidDotfilePath(ap) {
			_, _ = fmt.Fprintf(c.App.Writer, "\"%s\" is not a valid dotfile path", path)
			return nil
		}
		absPaths = append(absPaths, ap)
	}

	ai := snsync.AddInput{
		Session:  &session,
		Root:     root,
		Paths:    absPaths,
		PageSize: opts.pageSize,
		All:      c.Bool("all"),
	}

	var ao snsync.AddOutput

	ao, err = snsync.Add(ai, true)
	if err != nil {
		return err
	}

	if opts.display {
		_, _ = fmt.Fprintf(c.App.Writer, ao.Msg)
	}
	return err
}
