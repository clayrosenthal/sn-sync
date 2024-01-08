package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	snsync "github.com/clayrosenthal/sn-sync/sn-sync"

	"github.com/gookit/color"
	"github.com/spf13/viper"
	"github.com/urfave/cli/v2"
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

func printErrorMsg(c *cli.Context, err error) {
	_, _ = fmt.Fprintf(c.App.Writer, color.Red.Sprintf(err.Error()))
}

func printWarnMsg(c *cli.Context, msg string) {
	_, _ = fmt.Fprintf(c.App.Writer, color.Yellow.Sprintf(msg))
}

func printSuccessMsg(c *cli.Context, msg string) {
	_, _ = fmt.Fprintf(c.App.Writer, color.Green.Sprintf(msg))
}

func getOpts(c *cli.Context) (out configOptsOutput, err error) {
	out.useStdOut = c.Bool("no-stdout")

	if !c.Bool("no-stdout") {
		out.useStdOut = false
	}

	if c.Bool("use-session") || viper.GetBool("use_session") {
		out.useSession = true
	}

	out.sessKey = c.String("session-key")

	if viper.GetString("server") != "" {
		out.server = viper.GetString("server")
	}
	out.server = c.String("server")

	out.cacheDBDir = viper.GetString("cachedb_dir")
	if out.cacheDBDir != "" {
		out.cacheDBDir = c.String("cachedb-dir")
	}

	out.display = true
	if c.Bool("quiet") {
		out.display = false
	}

	out.home = c.String("home-dir")
	if out.home == "" {
		out.home = getHome()
	}

	out.pageSize = c.GlobalInt("page-size")

	out.debug = viper.GetBool("debug")
	if c.Bool("debug") {
		out.debug = true
	}

	return
}

func main() {
	if err := startCLI(os.Args); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	os.Exit(0)
}

func startCLI(args []string) (err error) {
	app := appSetup()

	sort.Sort(cli.FlagsByName(app.Flags))

	return app.Run(args)
}

func appSetup() (err error) {
	viper.SetEnvPrefix("sn")
	viper.AutomaticEnv()

	//TODO: figure out if these can be removed, what auto env does
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
	app.Authors = []*cli.Author{
		{
			Name:  "Jon Hadfield",
			Email: "jon@lessknown.co.uk",
		},
		{
			Name:  "Clay Rosenthal",
			Email: "contact@clayrosenthal.me",
		},
	}
	app.HelpName = "-"
	app.Usage = "Sync directories and files with Standard Notes"
	app.Description = ""
	app.BashComplete = func(c *cli.Context) {
		for _, cmd := range c.App.Commands {
			if !cmd.Hidden {
				fmt.Fprintln(c.App.Writer, cmd.Name)
			}
		}
	}

	app.Flags = []cli.Flag{
		&cli.BoolFlag{Name: "debug"},
		&cli.StringFlag{Name: "server"},
		&cli.BoolFlag{Name: "use-session"},
		&cli.StringFlag{Name: "session-key"},
		&cli.IntFlag{Name: "page-size", Hidden: true, Value: snsync.DefaultPageSize},
		&cli.BoolFlag{Name: "quiet"},
		&cli.BoolFlag{Name: "no-stdout"},
	}
	app.CommandNotFound = func(c *cli.Context, command string) {
		_, _ = fmt.Fprintf(c.App.Writer, "\ninvalid command: \"%s\" \n\n", command)
		cli.ShowAppHelpAndExit(c, 1)
	}

	app.Commands = []cli.Command{
		statusCmd(),
		addCmd(),
		diffCmd(),
		removeCmd(),
		syncCmd(),
		dotfilesCmd(),
		sessionCmd(),
		wipeCmd(),
	}

	return app
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
