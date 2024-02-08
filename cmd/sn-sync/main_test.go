package main

import (
	"bytes"
	"fmt"
	"index/suffixarray"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	snsync "github.com/clayrosenthal/sn-sync/sn-sync"
	"github.com/jonhadfield/gosn-v2/auth"
	"github.com/jonhadfield/gosn-v2/cache"
	"github.com/jonhadfield/gosn-v2/items"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func removeDB(dbPath string) {
	if err := os.Remove(dbPath); err != nil {
		if !strings.Contains(err.Error(), "no such file or directory") {
			panic(err)
		}
	}
}

func CleanUp(session cache.Session) error {
	removeDB(session.CacheDBPath)
	err := items.DeleteContent(&auth.Session{
		Token:             testCacheSession.Token,
		MasterKey:         testCacheSession.MasterKey,
		Server:            testCacheSession.Server,
		AccessToken:       testCacheSession.AccessToken,
		AccessExpiration:  testCacheSession.AccessExpiration,
		RefreshExpiration: testCacheSession.RefreshExpiration,
		RefreshToken:      testCacheSession.RefreshToken,
		Debug:             true,
	})
	return err
}

var testCacheSession *cache.Session

func csync(si cache.SyncInput) (so cache.SyncOutput, err error) {
	return cache.Sync(cache.SyncInput{
		Session: si.Session,
		Close:   si.Close,
	})
}
func TestMain(m *testing.M) {
	gs, err := auth.CliSignIn(os.Getenv("SN_EMAIL"), os.Getenv("SN_PASSWORD"), os.Getenv("SN_SERVER"), true)
	if err != nil {
		panic(err)
	}

	testCacheSession = &cache.Session{
		Session: &auth.Session{
			Debug:             true,
			Server:            gs.Server,
			Token:             gs.Token,
			MasterKey:         gs.MasterKey,
			RefreshExpiration: gs.RefreshExpiration,
			RefreshToken:      gs.RefreshToken,
			AccessToken:       gs.AccessToken,
			AccessExpiration:  gs.AccessExpiration,
		},
		CacheDBPath: "",
	}

	var path string

	path, err = cache.GenCacheDBPath(*testCacheSession, "", snsync.SNAppName)
	if err != nil {
		panic(err)
	}

	testCacheSession.CacheDBPath = path

	var so cache.SyncOutput
	so, err = csync(cache.SyncInput{
		Session: testCacheSession,
		Close:   false,
	})
	if err != nil {
		panic(err)
	}

	var allPersistedItems cache.Items

	if err = so.DB.All(&allPersistedItems); err != nil {
		return
	}
	if err = so.DB.Close(); err != nil {
		panic(err)
	}

	if testCacheSession.DefaultItemsKey.ItemsKey == "" {
		panic("failed in TestMain due to empty default items key")
	}
	os.Exit(m.Run())
}

func TestCLIInvalidCommand(t *testing.T) {
	var outputBuffer bytes.Buffer
	app := appSetup()
	app.Writer = &outputBuffer

	require.NoError(t, app.Run([]string{"sncli", "get", "tag", "--title", "missing tag"}))
	stdout := outputBuffer.String()
	fmt.Println(stdout)
	// Run the crashing code when FLAG is set
	if os.Getenv("FLAG") == "1" {
		require.NoError(t, app.Run([]string{"sn-sync", "lemon"}))
		return
	}
	// Run the test in a subprocess
	cmd := exec.Command(os.Args[0], "-test.run=TestCLIInvalidCommand")
	cmd.Env = append(os.Environ(), "FLAG=1")
	err := cmd.Run()

	// Cast the error as *exec.ExitError and compare the result
	e, ok := err.(*exec.ExitError)
	expectedErrorString := "exit status 1"
	assert.Equal(t, true, ok)
	assert.Equal(t, expectedErrorString, e.Error())
}

func TestStripHome(t *testing.T) {
	res, err := stripHome("/home/bob/something/else.txt", "/home/bob")
	assert.NoError(t, err)
	assert.Equal(t, "something/else.txt", res)
	res, err = stripHome("/home/bob/something/else.txt", "")
	assert.Error(t, err)
	assert.Empty(t, res)
	res, err = stripHome("", "/home/bob")
	assert.Error(t, err)
	assert.Empty(t, res)
}

func TestIsValidDotfilePath(t *testing.T) {
	home := getHome()
	assert.True(t, isValidDotfilePath(fmt.Sprintf("%s/.test", home)))
	assert.True(t, isValidDotfilePath(fmt.Sprintf("%s/.test/file.txt", home)))
	assert.True(t, isValidDotfilePath(fmt.Sprintf("%s/.test/test2/file.txt", home)))
	assert.False(t, isValidDotfilePath(fmt.Sprintf("%s/test/test2/file.txt", home)))
	assert.False(t, isValidDotfilePath(fmt.Sprintf("%s/test", home)))
}

func TestAdd(t *testing.T) {
	viper.SetEnvPrefix("sn")
	assert.NoError(t, viper.BindEnv("email"))
	assert.NoError(t, viper.BindEnv("password"))
	assert.NoError(t, viper.BindEnv("server"))
	serverURL := os.Getenv("SN_SERVER")
	if serverURL == "" {
		serverURL = snsync.SNServerURL
	}
	defer func() {
		if err := CleanUp(*testCacheSession); err != nil {
			fmt.Println("failed to wipe")
		}
	}()
	var err error
	home := getHome()
	fwc := make(map[string]string)
	applePath := fmt.Sprintf("%s/.fruit/apple", home)
	fwc[applePath] = "apple content"
	assert.NoError(t, createTemporaryFiles(fwc))
	var outputBuffer bytes.Buffer
	app := appSetup()
	app.Writer = &outputBuffer

	require.NoError(t, app.Run([]string{"sn-sync", "add", applePath}))
	stdout := outputBuffer.String()
	fmt.Println(stdout)
	assert.Regexp(t, regexp.MustCompile(".fruit/apple\\s*now tracked"), stdout)
}

func TestAddInvalidPath(t *testing.T) {
	var outputBuffer bytes.Buffer
	app := appSetup()
	app.Writer = &outputBuffer

	require.NoError(t, app.Run([]string{"sn-sync", "add", "/invalid"}))
	stdout := outputBuffer.String()
	fmt.Println(stdout)
	assert.Contains(t, stdout, "invalid")
}

func TestAddAllAndPath(t *testing.T) {
	var outputBuffer bytes.Buffer
	app := appSetup()
	app.Writer = &outputBuffer

	require.NoError(t, app.Run([]string{"sn-sync", "add", "--all", "/invalid"}))
	stdout := outputBuffer.String()
	fmt.Println(stdout)
	assert.Contains(t, stdout, "error: specifying --all and paths does not make sense")
}

func TestAddNoArgs(t *testing.T) {
	var outputBuffer bytes.Buffer
	app := appSetup()
	app.Writer = &outputBuffer

	require.NoError(t, app.Run([]string{"sn-sync", "add"}))
	stdout := outputBuffer.String()
	fmt.Println(stdout)
	assert.Contains(t, stdout, "error: either specify paths to add or --all to add everything")
}

func TestRemove(t *testing.T) {
	viper.SetEnvPrefix("sn")
	assert.NoError(t, viper.BindEnv("email"))
	assert.NoError(t, viper.BindEnv("password"))
	assert.NoError(t, viper.BindEnv("server"))

	home := getHome()
	fwc := make(map[string]string)
	applePath := fmt.Sprintf("%s/.fruit/apple", home)
	fwc[applePath] = "apple content"
	assert.NoError(t, createTemporaryFiles(fwc))

	defer func() {
		if err := CleanUp(*testCacheSession); err != nil {
			fmt.Println("failed to wipe")
		}
	}()

	var outputBuffer bytes.Buffer
	app := appSetup()
	app.Writer = &outputBuffer

	require.NoError(t, app.Run([]string{"sn-sync", "add", fmt.Sprintf("%s/.fruit", home)}))
	stdout := outputBuffer.String()
	fmt.Println(stdout)

	require.NoError(t, app.Run([]string{"sn-sync", "remove", fmt.Sprintf("%s/.fruit", home)}))
	stdout = outputBuffer.String()
	fmt.Println(stdout)

	assert.Regexp(t, regexp.MustCompile(".fruit/apple\\s*removed"), stdout)
}

func TestWipe(t *testing.T) {
	viper.SetEnvPrefix("sn")
	assert.NoError(t, viper.BindEnv("email"))
	assert.NoError(t, viper.BindEnv("password"))
	assert.NoError(t, viper.BindEnv("server"))

	home := getHome()
	fwc := make(map[string]string)
	applePath := fmt.Sprintf("%s/.fruit/apple", home)
	fwc[applePath] = "apple content"
	assert.NoError(t, createTemporaryFiles(fwc))
	serverURL := os.Getenv("SN_SERVER")
	if serverURL == "" {
		serverURL = snsync.SNServerURL
	}
	defer func() {
		if err := CleanUp(*testCacheSession); err != nil {
			fmt.Println("failed to wipe")
		}
	}()

	ai := snsync.AddInput{Session: testCacheSession, Home: home, Paths: []string{applePath}}
	_, err := snsync.Add(ai, true)
	assert.NoError(t, err)
	time.Sleep(time.Second * 1)
	var outputBuffer bytes.Buffer
	app := appSetup()
	app.Writer = &outputBuffer

	require.NoError(t, app.Run([]string{"sn-sync", "wipe", "--force"}))
	stdout := outputBuffer.String()
	fmt.Println(stdout)
	assert.Contains(t, stdout, "3 ")
}

func TestStatus(t *testing.T) {
	viper.SetEnvPrefix("sn")
	assert.NoError(t, viper.BindEnv("email"))
	assert.NoError(t, viper.BindEnv("password"))
	assert.NoError(t, viper.BindEnv("server"))

	var outputBuffer bytes.Buffer
	app := appSetup()
	app.Writer = &outputBuffer

	home := getHome()
	fwc := make(map[string]string)
	applePath := fmt.Sprintf("%s/.fruit/apple", home)
	fwc[applePath] = "apple content"
	assert.NoError(t, createTemporaryFiles(fwc))
	serverURL := os.Getenv("SN_SERVER")
	if serverURL == "" {
		serverURL = snsync.SNServerURL
	}
	defer func() {
		if err := CleanUp(*testCacheSession); err != nil {
			fmt.Println("failed to wipe")
		}
	}()
	var err error
	ai := snsync.AddInput{Session: testCacheSession, Home: home, Paths: []string{applePath}}
	_, err = snsync.Add(ai, true)
	assert.NoError(t, err)
	require.NoError(t, app.Run([]string{"sn-sync", "status", applePath}))
	stdout := outputBuffer.String()
	fmt.Println(stdout)
	assert.Contains(t, stdout, ".fruit/apple  identical")
}

func TestSync(t *testing.T) {
	viper.SetEnvPrefix("sn")
	assert.NoError(t, viper.BindEnv("email"))
	assert.NoError(t, viper.BindEnv("password"))
	assert.NoError(t, viper.BindEnv("server"))

	home := getHome()
	fwc := make(map[string]string)
	applePath := fmt.Sprintf("%s/.fruit/apple", home)
	fwc[applePath] = "apple content"
	lemonPath := fmt.Sprintf("%s/.fruit/lemon", home)
	fwc[lemonPath] = "lemon content"
	assert.NoError(t, createTemporaryFiles(fwc))
	serverURL := os.Getenv("SN_SERVER")
	if serverURL == "" {
		serverURL = snsync.SNServerURL
	}
	defer func() {
		if err := CleanUp(*testCacheSession); err != nil {
			fmt.Println("failed to wipe")
		}
	}()

	var err error
	ai := snsync.AddInput{Session: testCacheSession, Home: home, Paths: []string{applePath, lemonPath}}
	_, err = snsync.Add(ai, true)
	assert.NoError(t, err)

	var outputBuffer bytes.Buffer
	app := appSetup()
	app.Writer = &outputBuffer

	require.NoError(t, app.Run([]string{"sn-sync", "--debug", "sync", applePath}))
	stdout := outputBuffer.String()
	fmt.Println(stdout)
	assert.Contains(t, stdout, "nothing to do")
	// test push
	fwc[applePath] = "apple content updated"
	// add delay so local file is recognised as newer
	time.Sleep(1 * time.Second)
	assert.NoError(t, createTemporaryFiles(fwc))
	require.NoError(t, app.Run([]string{"sn-sync", "--debug", "sync", applePath}))
	stdout = outputBuffer.String()
	fmt.Println(stdout)
	assert.Contains(t, stdout, "pushed")
	// test pull - specify unchanged path and expect no change
	err = os.Remove(lemonPath)
	assert.NoError(t, err)
	require.NoError(t, app.Run([]string{"sn-sync", "--debug", "sync", applePath}))
	stdout = outputBuffer.String()
	fmt.Println(stdout)
	assert.Contains(t, stdout, "nothing to do")
	// test pull - specify changed path (updated content set to be older) and expect change

	fwc[lemonPath] = "lemon content updated"
	assert.NoError(t, createTemporaryFiles(fwc))

	tenMinsAgo := time.Now().Add(-time.Minute * 10)
	err = os.Chtimes(lemonPath, tenMinsAgo, tenMinsAgo)
	assert.NoError(t, err)
	require.NoError(t, app.Run([]string{"sn-sync", "--debug", "sync", lemonPath}))
	stdout = outputBuffer.String()
	fmt.Println(stdout)
	assert.NoError(t, err)
	r := regexp.MustCompile("pulled")
	index := suffixarray.New([]byte(stdout))
	results := index.FindAllIndex(r, -1)
	assert.Len(t, results, 1)
}

func TestDiff(t *testing.T) {
	viper.SetEnvPrefix("sn")
	assert.NoError(t, viper.BindEnv("email"))
	assert.NoError(t, viper.BindEnv("password"))
	assert.NoError(t, viper.BindEnv("server"))

	home := getHome()
	fwc := make(map[string]string)
	applePath := fmt.Sprintf("%s/.fruit/apple", home)
	fwc[applePath] = "apple content"
	assert.NoError(t, createTemporaryFiles(fwc))
	serverURL := os.Getenv("SN_SERVER")
	if serverURL == "" {
		serverURL = snsync.SNServerURL
	}
	defer func() {
		if err := CleanUp(*testCacheSession); err != nil {
			fmt.Println("failed to wipe")
		}
	}()
	ai := snsync.AddInput{Session: testCacheSession, Home: home, Paths: []string{applePath}}
	_, err := snsync.Add(ai, true)
	assert.NoError(t, err)

	var outputBuffer bytes.Buffer
	app := appSetup()
	app.Writer = &outputBuffer
	require.NoError(t, app.Run([]string{"sn-sync", "--debug", "diff", applePath}))
	stdout := outputBuffer.String()
	fmt.Println(stdout)
	assert.NoError(t, err)
	assert.NotEmpty(t, stdout)
	assert.Contains(t, stdout, "no differences")
	require.Error(t, app.Run([]string{"sn-sync", "--debug", "diff", "~/.does/not/exist"}))
	stdout = outputBuffer.String()
	fmt.Println(stdout)
	// might need a fix
	assert.Contains(t, err.Error(), "no such file")
}

func TestSyncExclude(t *testing.T) {
	viper.SetEnvPrefix("sn")
	assert.NoError(t, viper.BindEnv("email"))
	assert.NoError(t, viper.BindEnv("password"))
	assert.NoError(t, viper.BindEnv("server"))

	home := getHome()
	fwc := make(map[string]string)
	applePath := fmt.Sprintf("%s/.fruit/apple", home)
	fwc[applePath] = "apple content"
	assert.NoError(t, createTemporaryFiles(fwc))
	serverURL := os.Getenv("SN_SERVER")
	if serverURL == "" {
		serverURL = snsync.SNServerURL
	}
	defer func() {
		if err := CleanUp(*testCacheSession); err != nil {
			fmt.Println("failed to wipe")
		}
	}()

	var outputBuffer bytes.Buffer
	app := appSetup()
	app.Writer = &outputBuffer

	ai := snsync.AddInput{Session: testCacheSession, Home: home, Paths: []string{applePath}}
	_, err := snsync.Add(ai, true)
	assert.NoError(t, err)
	require.NoError(t, app.Run([]string{"sn-sync", "--debug", "sync", applePath}))
	stdout := outputBuffer.String()
	fmt.Println(stdout)
	assert.NoError(t, err)
	assert.Contains(t, stdout, "nothing to do")
	fwc[applePath] = "apple content updated"
	// add delay so local file is recognised as newer
	time.Sleep(1 * time.Second)
	assert.NoError(t, createTemporaryFiles(fwc))
	require.NoError(t, app.Run([]string{"sn-sync", "--debug", "sync", applePath}))
	stdout = outputBuffer.String()
	fmt.Println(stdout)
	assert.NoError(t, err)
	assert.Contains(t, stdout, "pushed")
}

func TestNumTrue(t *testing.T) {
	assert.Equal(t, 3, numTrue(true, false, true, true))
	assert.Equal(t, 0, numTrue())
}

func createPathWithContent(path, content string) error {
	dir, _ := filepath.Split(path)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	_, err = f.WriteString(content)
	if err != nil {
		return err
	}
	return f.Close()
}
func createTemporaryFiles(fwc map[string]string) error {
	for f, c := range fwc {
		if err := createPathWithContent(f, c); err != nil {
			return err
		}
	}
	return nil
}
