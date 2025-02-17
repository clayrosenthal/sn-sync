package snsync

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/jonhadfield/findexec"
	"github.com/jonhadfield/gosn-v2/cache"
	"github.com/jonhadfield/gosn-v2/items"
)

const (
	localMissing = "local missing"
	localNewer   = "local newer"
	remoteNewer  = "remote newer"
	untracked    = "untracked"
	identical    = "identical"
)

func Diff(session *cache.Session, home string, paths []string, pageSize int, close, useStdErr bool) (diffs []ItemDiff, msg string, err error) {
	debugPrint(session.Debug, fmt.Sprintf("Diff | %d paths", len(paths)))

	if !session.Debug {
		prefix := HiWhite("syncing ")
		if _, err = os.Stat(session.CacheDBPath); os.IsNotExist(err) {
			prefix = HiWhite("initializing ")
		}

		s := spinner.New(spinner.CharSets[SpinnerCharSet], SpinnerDelay*time.Millisecond, spinner.WithWriter(os.Stdout))
		if useStdErr {
			s = spinner.New(spinner.CharSets[SpinnerCharSet], SpinnerDelay*time.Millisecond, spinner.WithWriter(os.Stderr))
		}

		s.Prefix = prefix
		s.Start()
		defer s.Stop()
	}

	// get populated db
	si := cache.SyncInput{
		Session: session,
		Close:   false,
	}
	var cso cache.SyncOutput
	cso, err = cache.Sync(si)
	if err != nil {
		return
	}

	var remote tagsWithNotes

	remote, err = getTagsWithNotes(cso.DB, session)
	if err != nil {
		return diffs, msg, err
	}
	if err = cso.DB.Close(); err != nil {
		return
	}

	return diff(remote, home, paths, session.Debug)
}

// TODO: rename homeRelPath? relPath? rootRelPath?
type ItemDiff struct {
	tagTitle    string
	noteTitle   string
	path        string
	homeRelPath string
	diff        string
	remote      items.Note
	local       string
}

func diff(twn tagsWithNotes, home string, paths []string, debug bool) (diffs []ItemDiff, msg string, err error) {
	debugPrint(debug, fmt.Sprintf("diff | %d remote items", len(twn)))

	err = checkNoteTagConflicts(twn)
	if err != nil {
		return
	}

	if len(twn) == 0 {
		msg = "no sync being tracked"
		return
	}

	if len(paths) == 0 {
		debugPrint(debug, fmt.Sprint("diff | calling compare without any Paths"))
	} else {
		debugPrint(debug, fmt.Sprintf("diff | calling compare with Paths: %s", strings.Join(paths, ",")))
	}

	diffs, err = compare(twn, home, paths, []string{}, debug)
	if err != nil {
		return diffs, msg, err
	}

	debugPrint(debug, fmt.Sprintf("compare | %d diffs generated", len(diffs)))

	if len(diffs) == 0 {
		return diffs, msg, err
	}

	diffBinary := findexec.Find("diff", "")
	if diffBinary == "" {
		err = errors.New("failed to find compare binary")
		return
	}

	var differencesFound bool
	// getTagsWithNotes tempdir
	tempDir := os.TempDir()
	if !strings.HasSuffix(tempDir, string(os.PathSeparator)) {
		tempDir += string(os.PathSeparator)
	}

	differencesFound, err = processContentDiffs(diffs, tempDir, diffBinary)
	if err != nil {
		return
	}

	if !differencesFound {
		msg = "no differences found"
	}

	return diffs, msg, err
}

func processContentDiffs(diffs []ItemDiff, tempDir, diffBinary string) (differencesFound bool, err error) {
	for _, diff := range diffs {
		localContent := diff.local

		remoteContent := diff.remote.Content.GetText()
		if localContent != remoteContent {
			differencesFound = true
			// write local and remote content to temporary files
			var f1, f2 *os.File

			uuid := items.GenUUID()
			f1path := fmt.Sprintf("%ssn-sync-compare-%s-f1", tempDir, uuid)
			f2path := fmt.Sprintf("%ssn-sync-compare-%s-f2", tempDir, uuid)

			f1, err = os.Create(f1path)
			if err != nil {
				return
			}

			f2, err = os.Create(f2path)
			if err != nil {
				return
			}

			if _, err = f1.WriteString(diff.local); err != nil {
				return
			}

			if _, err = f2.WriteString(diff.remote.Content.GetText()); err != nil {
				return
			}

			cmd := exec.Command(diffBinary, f1path, f2path)
			out, oErr := cmd.CombinedOutput()

			if err = os.Remove(f1path); err != nil {
				return
			}

			if err = os.Remove(f2path); err != nil {
				return
			}

			var exitCode int

			if oErr != nil {
				if exitError, ok := oErr.(*exec.ExitError); ok {
					exitCode = exitError.ExitCode()
				}
			}

			if exitCode == 2 {
				panic(fmt.Sprintf("failed to compare: '%s' with '%s'", f1path, f2path))
			}

			fmt.Println(bold(diff.homeRelPath))
			fmt.Println(string(out))
		}
	}

	return differencesFound, err
}

func pathIsPrefixOfPaths(path string, paths []string) bool {
	for i := range paths {
		inSliceDIR, _ := filepath.Split(paths[i])
		if inSliceDIR == "" {
			continue
		}

		if path == inSliceDIR || strings.HasPrefix(path, inSliceDIR) {
			return true
		}
	}

	return false
}

func noteInPaths(note string, paths []string) bool {
	if note == "" || len(paths) == 0 {
		return false
	}

	for i := range paths {
		if paths[i] == "" {
			continue
		}

		if note == paths[i] {
			return true
		}

		d, _ := filepath.Split(note)
		if d == paths[i] {
			return true
		}

		rel, err := filepath.Rel(paths[i], note)
		if err == nil && !strings.HasPrefix(rel, "../") {
			return true
		}
	}

	return false
}

func checkPathsExist(paths []string) error {
	for _, p := range paths {
		if _, err := os.Stat(p); err != nil || os.IsNotExist(err) {
			return fmt.Errorf("failed to read path: %s", p)
		}
	}

	return nil
}

func tagExists(title string, twn tagsWithNotes) bool {
	for _, twn := range twn {
		if twn.tag.Content.GetTitle() == title {
			return true
		}
	}

	return false
}

func findUntracked(paths, existingRemoteEquivalentPaths []string, home string, debug bool) (itemDiffs []ItemDiff) {
	// if path is directory, then walk to generate list of additional Paths
	for _, path := range paths {
		debugPrint(debug, fmt.Sprintf("compare | diffing path: %s", stripHome(path, home)))

		if StringInSlice(path, existingRemoteEquivalentPaths, true) {
			continue
		}

		if stat, err := os.Stat(path); err == nil && stat.IsDir() {
			debugPrint(debug, fmt.Sprintf("compare | walking path: %s", path))

			err = filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
				// don't check tracked Paths
				if StringInSlice(p, existingRemoteEquivalentPaths, true) {
					return nil
				}
				if err != nil {
					fmt.Printf("failed to read path %q: %v\n", p, err)
					return err
				}
				// ensure walked path is valid
				if v, err := pathValid(p); !v {
					return err
				}
				// add file as untracked
				if stat, err := os.Stat(p); err == nil && !stat.IsDir() {
					debugPrint(debug, fmt.Sprintf("compare | file is untracked: %s", p))
					homeRelPath := stripHome(p, home)
					itemDiffs = append(itemDiffs, ItemDiff{
						homeRelPath: homeRelPath,
						path:        p,
						diff:        untracked,
					})
				}
				return nil
			})
			if err != nil {
				return
			}
		} else {
			homeRelPath := stripHome(path, home)
			debugPrint(debug, fmt.Sprintf("compare | file is untracked: %s", path))

			itemDiffs = append(itemDiffs, ItemDiff{
				homeRelPath: homeRelPath,
				path:        path,
				diff:        untracked,
			})
		}
	}

	return itemDiffs
}
