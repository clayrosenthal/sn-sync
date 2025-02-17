package snsync

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/asdine/storm/v3"
	"github.com/briandowns/spinner"
	"github.com/jonhadfield/gosn-v2/cache"
	"github.com/jonhadfield/gosn-v2/items"
	"github.com/ryanuber/columnize"
)

// Add tracks local Paths by pushing the local dir as a tag representation and the filename as a note title
func Add(ai AddInput, useStdErr bool) (ao AddOutput, err error) {
	// validate session
	if !ai.Session.Valid() {
		err = errors.New("invalid session")
		return
	}

	if StringInSlice(ai.Home, []string{"/", "/home"}, true) {
		err = errors.New(fmt.Sprintf("not a good idea to use '%s' as home dir", ai.Home))
		return
	}

	var noRecurse bool
	if ai.All {
		noRecurse = true

		ai.Paths, err = discoverDotfilesInHome(ai.Home, ai.Session.Debug)
		if err != nil {
			return
		}
	}

	// check paths defined
	if len(ai.Paths) == 0 {
		return ao, errors.New("paths not defined")
	}

	ai.Paths, err = preflight(ai.Home, ai.Paths)
	if err != nil {
		return
	}

	debugPrint(ai.Session.Debug, fmt.Sprintf("Add | paths after dedupe: %d", len(ai.Paths)))

	if !ai.Session.Debug {
		prefix := HiWhite("syncing ")
		if _, err = os.Stat(ai.Session.CacheDBPath); os.IsNotExist(err) {
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
		Session: ai.Session,
		Close:   false,
	}

	var cso cache.SyncOutput

	cso, err = cache.Sync(si)
	if err != nil {
		return
	}

	var twn tagsWithNotes

	twn, err = getTagsWithNotes(cso.DB, ai.Session)
	if err != nil {
		return
	}
	// run pre-checks
	err = checkNoteTagConflicts(twn)
	if err != nil {
		return
	}

	ai.Twn = twn

	ao, err = add(cso.DB, ai, noRecurse)
	si.CacheDB = cso.DB
	// syncDBwithFS db back to SN
	si.Close = true
	cso, err = cache.Sync(si)

	return
}

type AddInput struct {
	Session  *cache.Session
	Home     string
	Paths    []string
	All      bool
	Twn      tagsWithNotes
	PageSize int
}

type AddOutput struct {
	TagsPushed, NotesPushed                 int
	PathsAdded, PathsExisting, PathsInvalid []string
	Msg                                     string
}

func add(db *storm.DB, ai AddInput, noRecurse bool) (ao AddOutput, err error) {
	var tagToItemMap map[string]items.Items

	var fsPathsToAdd []string

	// generate list of Paths to add
	fsPathsToAdd, err = getLocalFSPaths(ai.Paths, noRecurse)
	if err != nil {
		return
	}

	if len(fsPathsToAdd) == 0 {
		return
	}

	var statusLines []string

	statusLines, tagToItemMap, ao.PathsAdded, ao.PathsExisting, err = generateTagItemMap(fsPathsToAdd, ai.Home, ai.Twn)
	if err != nil {
		return
	}
	// add DotFilesTag tag if missing
	_, dotFilesTagInTagToItemMap := tagToItemMap[DotFilesTag]
	if !tagExists("sync", ai.Twn) && !dotFilesTagInTagToItemMap {
		debugPrint(ai.Session.Debug, "Add | adding missing sync tag")

		tagToItemMap[DotFilesTag] = items.Items{}
	}

	// addToDB and tag items
	ao.TagsPushed, ao.NotesPushed, err = pushAndTag(db, ai.Session, tagToItemMap, ai.Twn)
	if err != nil {
		return
	}

	debugPrint(ai.Session.Debug, fmt.Sprintf("Add | tags pushed: %d notes pushed %d", ao.TagsPushed, ao.NotesPushed))

	ao.Msg = fmt.Sprint(columnize.SimpleFormat(statusLines))

	return ao, err
}

func generateTagItemMap(fsPaths []string, home string, twn tagsWithNotes) (statusLines []string,
	tagToItemMap map[string]items.Items, pathsAdded, pathsExisting []string, err error) {
	tagToItemMap = make(map[string]items.Items)

	var added []string

	var existing []string

	for _, path := range fsPaths {
		dir, filename := filepath.Split(path)
		homeRelPath := stripHome(dir+filename, home)
		boldHomeRelPath := bold(homeRelPath)

		var remoteTagTitleWithoutHome, remoteTagTitle string
		remoteTagTitleWithoutHome = stripHome(dir, home)
		remoteTagTitle = pathToTag(remoteTagTitleWithoutHome)

		existingCount := noteWithTagExists(remoteTagTitle, filename, twn)
		if existingCount > 0 {
			existing = append(existing, fmt.Sprintf("%s | %s", boldHomeRelPath, yellow("already tracked")))
			pathsExisting = append(pathsExisting, path)

			continue
		} else if existingCount > 1 {
			err = fmt.Errorf("duplicate items found with name '%s' and tag '%s'", filename, remoteTagTitle)
			return statusLines, tagToItemMap, pathsAdded, pathsExisting, err
		}
		// now add
		pathsAdded = append(pathsAdded, path)

		var itemToAdd items.Note

		itemToAdd, err = createItem(path, filename)
		if err != nil {
			return
		}

		tagToItemMap[remoteTagTitle] = append(tagToItemMap[remoteTagTitle], &itemToAdd)
		added = append(added, fmt.Sprintf("%s | %s", boldHomeRelPath, green("now tracked")))
	}

	statusLines = append(statusLines, existing...)
	statusLines = append(statusLines, added...)

	return statusLines, tagToItemMap, pathsAdded, pathsExisting, err
}

func getLocalFSPaths(paths []string, noRecurse bool) (finalPaths []string, err error) {
	// check for directories
	for _, path := range paths {
		// if path is directory, then walk to generate list of additional Paths
		var stat os.FileInfo
		if stat, err = os.Stat(path); err == nil && stat.IsDir() && !noRecurse {
			err = filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return fmt.Errorf("failed to read path %q: %v", path, err)
				}
				stat, err = os.Stat(path)
				if err != nil {
					return err
				}
				// if it's a dir, then carry on
				if stat.IsDir() {
					return nil
				}

				// if file is valid, then add
				var valid bool
				valid, err = pathValid(path)
				if err != nil {
					return err
				}
				if valid {
					finalPaths = append(finalPaths, path)
					return err
				}
				return nil
			})
			// return if we failed to walk the dir
			if err != nil {
				return
			}
		} else {
			// path is file
			var valid bool
			valid, err = pathValid(path)
			if err != nil {
				return
			}
			if valid {
				finalPaths = append(finalPaths, path)
			}
		}
	}
	// dedupe
	finalPaths = dedupe(finalPaths)

	return finalPaths, err
}

func createItem(path, title string) (item items.Note, err error) {
	// read file content
	var file *os.File

	file, err = os.Open(path)
	if err != nil {
		return
	}

	defer func() {
		if err = file.Close(); err != nil {
			fmt.Println("failed to close file:", path)
		}
	}()

	var localBytes []byte

	localBytes, err = io.ReadAll(file)
	if err != nil {
		return
	}

	//TODO fill in references
	var references items.ItemReferences

	localStr := string(localBytes)
	// addToDB item
	item, err = items.NewNote(title, localStr, references)
	if err != nil {
		return
	}
	item.Content.SetPrefersPlainEditor(true)

	return item, err
}

func pathInfo(path string) (mode os.FileMode, pathSize int64, err error) {
	var fi os.FileInfo

	fi, err = os.Lstat(path)
	if err != nil {
		return
	}

	mode = fi.Mode()
	if mode.IsRegular() {
		pathSize = fi.Size()
	}

	return
}

func discoverDotfilesInHome(home string, debug bool) (paths []string, err error) {
	debugPrint(debug, fmt.Sprintf("discoverDotfilesInHome | checking home: %s", home))

	var homeEntries []os.DirEntry

	homeEntries, err = os.ReadDir(home)
	if err != nil {
		return
	}

	for _, entry := range homeEntries {
		if strings.HasPrefix(entry.Name(), ".") {
			var absoluteFilePath string
			var info os.FileInfo

			absoluteFilePath, err = filepath.Abs(home + string(os.PathSeparator) + entry.Name())
			if err != nil {
				return
			}

			info, err = os.Lstat(absoluteFilePath)
			if err != nil {
				return
			}

			if info.Mode().IsRegular() {
				paths = append(paths, absoluteFilePath)
			}
		}
	}

	return
}

func pathValid(path string) (valid bool, err error) {
	var mode os.FileMode

	var pSize int64

	mode, pSize, err = pathInfo(path)
	if err != nil {
		return
	}

	switch {
	case mode.IsRegular():
		if pSize > 10240000 {
			err = fmt.Errorf("file too large: %s", path)
			return false, err
		}

		return true, nil
	case mode&os.ModeSymlink != 0:
		return false, fmt.Errorf("symlink not supported: %s", path)
	case mode.IsDir():
		return true, nil
	case mode&os.ModeSocket != 0:
		return false, fmt.Errorf("sockets not supported: %s", path)
	case mode&os.ModeCharDevice != 0:
		return false, fmt.Errorf("char device file not supported: %s", path)
	case mode&os.ModeDevice != 0:
		return false, fmt.Errorf("device file not supported: %s", path)
	case mode&os.ModeNamedPipe != 0:
		return false, fmt.Errorf("named pipe not supported: %s", path)
	case mode&os.ModeTemporary != 0:
		return false, fmt.Errorf("temporary file not supported: %s", path)
	case mode&os.ModeIrregular != 0:
		return false, fmt.Errorf("irregular file not supported: %s", path)
	default:
		return false, fmt.Errorf("unknown file type: %s", path)
	}
}
