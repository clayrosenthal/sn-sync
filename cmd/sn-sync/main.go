package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jonhadfield/gosn-v2/cache"

	snsync "github.com/clayrosenthal/sn-sync/sn-sync"

	"github.com/spf13/viper"
	"github.com/urfave/cli"
)

// overwritten at build time
var version, versionOutput, tag, sha, buildDate string

type configOptsOutput struct {
	useStdOut  bool
	display    bool
	useSession bool
	home       string
	sessKey    string
	server     string
	pageSize   int
	cacheDBDir string
	debug      bool
}

func getOpts(c *cli.Context) (out configOptsOutput, err error) {
	out.useStdOut = c.Bool("no-stdout")

	if !c.GlobalBool("no-stdout") {
		out.useStdOut = false
	}

	if c.GlobalBool("use-session") || viper.GetBool("use_session") {
		out.useSession = true
	}

	out.sessKey = c.GlobalString("session-key")

	out.server = c.GlobalString("server")
	if viper.GetString("server") != "" {
		out.server = viper.GetString("server")
	}

	out.cacheDBDir = viper.GetString("cachedb_dir")
	if out.cacheDBDir != "" {
		out.cacheDBDir = c.GlobalString("cachedb-dir")
	}

	out.display = true
	if c.GlobalBool("quiet") {
		out.display = false
	}

	out.home = c.GlobalString("home-dir")
	if out.home == "" {
		out.home = getHome()
	}

	out.pageSize = c.GlobalInt("page-size")

	out.debug = viper.GetBool("debug")
	if c.GlobalBool("debug") {
		out.debug = true
	}

	return
}

func main() {
	msg, display, err := startCLI(os.Args)
	if err != nil {
		fmt.Printf("error: %+v\n", err)
		os.Exit(1)
	}

	if display && msg != "" {
		fmt.Println(msg)
	}

	os.Exit(0)
}

func startCLI(args []string) (msg string, display bool, err error) {
	viper.SetEnvPrefix("sn")

	err = viper.BindEnv("email")
	if err != nil {
		return "", false, err
	}

	err = viper.BindEnv("password")
	if err != nil {
		return "", false, err
	}

	err = viper.BindEnv("server")
	if err != nil {
		return "", false, err
	}

	err = viper.BindEnv("debug")
	if err != nil {
		return "", false, err
	}

	err = viper.BindEnv("use_session")
	if err != nil {
		return "", false, err
	}

	if tag != "" && buildDate != "" {
		versionOutput = fmt.Sprintf("[%s-%s] %s UTC", tag, sha, buildDate)
	} else {
		versionOutput = version
	}

	app := cli.NewApp()
	app.EnableBashCompletion = true

	app.Name = "sn-sync"
	app.Version = versionOutput
	app.Compiled = time.Now()
	app.Authors = []cli.Author{
		{
			Name:  "Jon Hadfield",
			Email: "jon@lessknown.co.uk",
		},
	}
	app.HelpName = "-"
	app.Usage = "sync sync with Standard Notes"
	app.Description = ""

	app.Flags = []cli.Flag{
		cli.BoolFlag{Name: "debug"},
		cli.StringFlag{Name: "server"},
		cli.StringFlag{Name: "home-dir"},
		cli.BoolFlag{Name: "use-session"},
		cli.StringFlag{Name: "session-key"},
		cli.IntFlag{Name: "page-size", Hidden: true, Value: snsync.DefaultPageSize},
		cli.BoolFlag{Name: "quiet"},
		cli.BoolFlag{Name: "no-stdout"},
	}
	app.CommandNotFound = func(c *cli.Context, command string) {
		_, _ = fmt.Fprintf(c.App.Writer, "\ninvalid command: \"%s\" \n\n", command)
		cli.ShowAppHelpAndExit(c, 1)
	}
	statusCmd := cli.Command{
		Name:  "status",
		Usage: "compare local and remote",
		Action: func(c *cli.Context) error {
			var opts configOptsOutput
			opts, err = getOpts(c)
			if err != nil {
				return err
			}
			display = opts.display

			var session cache.Session
			session, _, err = cache.GetSession(opts.useSession, opts.sessKey, opts.server, opts.debug)

			var cacheDBPath string
			cacheDBPath, err = cache.GenCacheDBPath(session, opts.cacheDBDir, snsync.SNAppName)
			if err != nil {
				return err
			}
			session.CacheDBPath = cacheDBPath

			_, msg, err = snsync.Status(&session, opts.home, c.Args(), opts.pageSize, opts.debug, false)
			return err
		},
	}

	syncCmd := cli.Command{
		Name:  "sync",
		Usage: "sync sync",
		Flags: []cli.Flag{
			cli.StringSliceFlag{
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
			var opts configOptsOutput
			opts, err = getOpts(c)
			if err != nil {
				return err
			}
			display = opts.display

			var session cache.Session
			session, _, err = cache.GetSession(opts.useSession,
				opts.sessKey, opts.server, opts.debug)
			var cacheDBPath string
			cacheDBPath, err = cache.GenCacheDBPath(session, opts.cacheDBDir, snsync.SNAppName)
			if err != nil {
				return err
			}
			session.CacheDBPath = cacheDBPath

			var so snsync.SyncOutput
			so, err = snsync.Sync(snsync.SNDotfilesSyncInput{
				Session:  &session,
				Home:     opts.home,
				Paths:    c.Args(),
				Exclude:  c.StringSlice("exclude"),
				PageSize: opts.pageSize,
				Debug:    opts.debug,
			}, c.GlobalBool("no-stdout"))

			if err != nil {
				return err
			}
			msg = so.Msg

			return err
		},
	}

	addCmd := cli.Command{
		Name:  "add",
		Usage: "start tracking file(s)",
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name:  "all",
				Usage: "add all sync (non-recursive)",
			},
		},
		Action: func(c *cli.Context) error {
			var opts configOptsOutput
			opts, err = getOpts(c)
			if err != nil {
				return err
			}
			display = opts.display

			if !c.Bool("all") && len(c.Args()) == 0 {
				msg = "error: either specify paths to add or --all to add everything"
				_ = cli.ShowCommandHelp(c, "add")
				return nil
			}

			if c.Bool("all") && len(c.Args()) > 0 {
				msg = "error: specifying --all and paths does not make sense"
				_ = cli.ShowCommandHelp(c, "add")
				return nil
			}

			var absPaths []string
			for _, path := range c.Args() {
				var ap string
				ap, err = filepath.Abs(path)
				if err != nil {
					return err
				}
				if !isValidDotfilePath(ap) {
					msg = fmt.Sprintf("\"%s\" is not a valid dotfile path", path)
					return nil
				}
				absPaths = append(absPaths, ap)
			}

			var session cache.Session
			session, _, err = cache.GetSession(opts.useSession,
				opts.sessKey, opts.server, opts.debug)
			var cacheDBPath string
			cacheDBPath, err = cache.GenCacheDBPath(session, opts.cacheDBDir, snsync.SNAppName)
			if err != nil {
				return err
			}
			session.CacheDBPath = cacheDBPath

			ai := snsync.AddInput{Session: &session, Home: opts.home, Paths: absPaths,
				PageSize: opts.pageSize, All: c.Bool("all")}

			var ao snsync.AddOutput

			ao, err = snsync.Add(ai, true)
			if err != nil {
				return err
			}

			msg = ao.Msg

			return err
		},
	}

	removeCmd := cli.Command{
		Name:  "remove",
		Usage: "stop tracking file(s)",
		Action: func(c *cli.Context) error {
			if len(c.Args()) == 0 {
				_ = cli.ShowCommandHelp(c, "remove")
				return nil
			}

			var opts configOptsOutput
			opts, err = getOpts(c)
			if err != nil {
				return err
			}
			display = opts.display

			if len(c.Args()) == 0 {
				msg = "error: paths not specified"
				_ = cli.ShowCommandHelp(c, "add")
				return nil
			}

			var session cache.Session
			session, _, err = cache.GetSession(opts.useSession,
				opts.sessKey, opts.server,
				opts.debug)
			var cacheDBPath string
			cacheDBPath, err = cache.GenCacheDBPath(session, opts.cacheDBDir, snsync.SNAppName)
			if err != nil {
				return err
			}
			session.CacheDBPath = cacheDBPath

			ri := snsync.RemoveInput{
				Session:  &session,
				Home:     opts.home,
				Paths:    c.Args(),
				PageSize: opts.pageSize,
				Debug:    opts.debug,
			}

			var ro snsync.RemoveOutput

			ro, err = snsync.Remove(ri, c.Bool("no-stdout"))
			if err != nil {
				return err
			}
			msg = ro.Msg

			return err
		},
	}

	diffCmd := cli.Command{
		Name:  "diff",
		Usage: "display differences between local and remote",
		Action: func(c *cli.Context) error {
			var opts configOptsOutput
			opts, err = getOpts(c)
			if err != nil {
				return err
			}
			display = opts.display

			var session cache.Session
			session, _, err = cache.GetSession(opts.useSession,
				opts.sessKey, opts.server, opts.debug)

			var cacheDBPath string

			cacheDBPath, err = cache.GenCacheDBPath(session, opts.cacheDBDir, snsync.SNAppName)
			if err != nil {
				return err
			}

			session.CacheDBPath = cacheDBPath

			_, msg, err = snsync.Diff(&session, opts.home, c.Args(), opts.pageSize, true, c.Bool("no-stdout"))

			return err
		},
	}

	sessionCmd := cli.Command{
		Name:  "session",
		Usage: "manage session credentials",
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name:  "add",
				Usage: "add session to keychain",
			},
			cli.BoolFlag{
				Name:  "remove",
				Usage: "remove session from keychain",
			},
			cli.BoolFlag{
				Name:  "status",
				Usage: "get session details",
			},
			cli.StringFlag{
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
		Action: func(c *cli.Context) error {
			var opts configOptsOutput
			opts, err = getOpts(c)
			if err != nil {
				return err
			}
			display = opts.display

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
				msg, err = auth.AddSession(opts.server, sessKey, nil, c.Bool("debug"))
				return err
			}
			if sRemove {
				msg = auth.RemoveSession(nil)
				return nil
			}
			if sStatus {
				msg, err = auth.SessionStatus(sessKey, nil)
			}
			return err
		},
	}

	wipeCmd := cli.Command{
		Name:  "wipe",
		Usage: "remove all sync",
		Flags: []cli.Flag{
			cli.BoolFlag{
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
			var opts configOptsOutput
			opts, err = getOpts(c)
			if err != nil {
				return err
			}
			display = opts.display

			var email string
			var session cache.Session
			session, email, err = cache.GetSession(opts.useSession,
				opts.sessKey, opts.server,
				opts.debug)
			var cacheDBPath string
			cacheDBPath, err = cache.GenCacheDBPath(session, opts.cacheDBDir, snsync.SNAppName)
			if err != nil {
				return err
			}
			session.CacheDBPath = cacheDBPath

			var proceed bool
			if c.Bool("force") {
				proceed = true
			} else {
				fmt.Printf("wipe all sync for account %s? ", email)
				var input string
				_, err = fmt.Scanln(&input)
				if err == nil && snsync.StringInSlice(input, []string{"y", "yes"}, false) {
					proceed = true
				}
			}
			if proceed {
				var num int
				num, err = snsync.WipeDotfileTagsAndNotes(&session, opts.pageSize, c.Bool("no-stdout"))
				if err != nil {
					return err
				}
				msg = fmt.Sprintf("%d removed", num)
			} else {
				return nil
			}

			return err
		},
	}

	app.Commands = []cli.Command{
		statusCmd,
		syncCmd,
		addCmd,
		removeCmd,
		diffCmd,
		sessionCmd,
		wipeCmd,
	}

	sort.Sort(cli.FlagsByName(app.Flags))

	return msg, display, app.Run(args)
}

func numTrue(in ...bool) (total int) {
	for _, i := range in {
		if i {
			total++
		}
	}

	return
}

func stripHome(in, home string) (res string, err error) {
	if home == "" {
		err = errors.New("home required")
		return
	}

	if in == "" {
		err = errors.New("path required")
		return
	}

	if in == home {
		return
	}

	if strings.HasPrefix(in, home) {
		return in[len(home)+1:], nil
	}

	return
}

func getHome() string {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("failed to get home directory")
		panic(err)
	}

	return home
}

func isValidDotfilePath(path string) bool {
	home := getHome()

	dir, filename := filepath.Split(path)

	homeRelPath, err := stripHome(dir+filename, home)
	if err != nil {
		return false
	}

	return strings.HasPrefix(homeRelPath, ".")
}
