package main

import (
	"fmt"
	"os"

	snsync "github.com/clayrosenthal/sn-sync/sn-sync"
	"github.com/jonhadfield/gosn-v2/cache"
	"github.com/jonhadfield/gosn-v2/session"
	cli "github.com/urfave/cli/v2"
)

func sessionCmd() *cli.Command {
	return &cli.Command{
		Name:  "session",
		Usage: "manage session credentials",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "add",
				Usage: "add session to keychain",
			},
			&cli.BoolFlag{
				Name:  "remove",
				Usage: "remove session from keychain",
			},
			&cli.BoolFlag{
				Name:  "status",
				Usage: "get session details",
			},
			&cli.StringFlag{
				Name:     "session-key",
				Usage:    "[optional] key to encrypt/decrypt session (enter '.' to hide key input)",
				Required: false,
			},
		},
		Hidden: false,
		BashComplete: func(c *cli.Context) {
			tasks := []string{"--add", "--remove", "--status", "--session-key"}
			if c.NArg() > 0 {
				return
			}
			for _, t := range tasks {
				fmt.Println(t)
			}
		},
		Action: func(c *cli.Context) (err error) {
			var opts configOptsOutput
			opts, err = getOpts(c)
			if err != nil {
				return err
			}

			sAdd := c.Bool("add")
			sRemove := c.Bool("remove")
			sStatus := c.Bool("status")
			sessKey := c.String("session-key")

			nTrue := numTrue(sAdd, sRemove, sStatus)
			if nTrue == 0 || nTrue > 1 {
				_ = cli.ShowCommandHelp(c, "session")
				os.Exit(1)
			}
			if sAdd {
				msg, err := session.AddSession(opts.server, sessKey, nil, c.Bool("debug"))
				_, _ = fmt.Fprintf(c.App.Writer, msg)
				return err
			}
			if sRemove {
				msg := session.RemoveSession(nil)
				_, _ = fmt.Fprintf(c.App.Writer, msg)
				return nil
			}
			if sStatus {
				msg, err := session.SessionStatus(sessKey, nil)
				_, _ = fmt.Fprintf(c.App.Writer, msg)
				return err
			}
			return err
		},
	}
}

func getSession(opts configOptsOutput) (session cache.Session, err error) {
	session, _, err = cache.GetSession(opts.useSession,
		opts.sessKey, opts.server, opts.debug)

	var cacheDBPath string

	cacheDBPath, err = cache.GenCacheDBPath(session, opts.cacheDBDir, snsync.SNAppName)
	if err != nil {
		return
	}

	session.CacheDBPath = cacheDBPath

	return
}
