package snsync

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/asdine/storm/v3"
	"github.com/briandowns/spinner"
	"github.com/fatih/color"
	"github.com/jonhadfield/gosn-v2/cache"

	"github.com/ryanuber/columnize"
)

var (
	HiWhite = color.New(color.FgHiWhite).SprintFunc()
)

// Sync compares local and remote items and then:
// - pulls remotes if locals are older or missing
// - pushes locals if remotes are newer
func Sync(si SNDirSyncInput, useStdErr bool) (so SyncOutput, err error) {
	if err = checkPathsExist(si.Exclude); err != nil {
		return
	}

	if !si.Debug {
		prefix := HiWhite("syncing ")
		if _, err = os.Stat(si.Session.CacheDBPath); os.IsNotExist(err) {
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

	output, err := sync(syncInput{
		session: si.Session,
		root:    si.Root,
		paths:   si.Paths,
		exclude: si.Exclude,
		debug:   si.Debug,
		close:   false,
	})

	return SyncOutput{
		NoPushed: output.noPushed,
		NoPulled: output.noPulled,
		Msg:      output.msg,
	}, err
}

func sync(input syncInput) (output syncOutput, err error) {
	// get populated db
	csi := cache.SyncInput{
		Session: input.session,
		Close:   false,
	}

	var cso cache.SyncOutput
	cso, err = cache.Sync(csi)
	if err != nil {
		return
	}

	var remote tagsWithNotes
	remote, err = getTagsWithNotes(cso.DB, input.session)
	if err != nil {
		return
	}

	err = checkNoteTagConflicts(remote)
	if err != nil {
		return
	}

	output, err = syncDBwithFS(syncInput{
		db:      cso.DB,
		session: input.session,
		twn:     remote,
		root:    input.root,
		paths:   input.paths,
		exclude: input.exclude,
		debug:   input.debug})
	if err != nil {

		return
	}

	if err = cso.DB.Close(); err != nil {
		return
	}

	// TODO: Check every editor component and ensure no sync are associated (ensure plain text editor)

	// persist changes
	csi.Close = true
	_, err = cache.Sync(csi)

	return
}

type SNDirSyncInput struct {
	Session        *cache.Session
	Root           string
	Paths, Exclude []string
	PageSize       int
	Debug          bool
}

type SyncOutput struct {
	NoPushed, NoPulled int
	Msg                string
}

func syncDBwithFS(si syncInput) (so syncOutput, err error) {
	if si.db == nil {
		panic("didn't get db sent to syncDBwithFS")
	}
	var itemDiffs []ItemDiff

	itemDiffs, err = compare(si.twn, si.root, si.paths, si.exclude, si.debug)
	if err != nil {
		if strings.Contains(err.Error(), "tags with notes not supplied") {
			err = errors.New("no remote sync found")
		}

		return
	}

	var itemsToPush, itemsToPull []ItemDiff

	var itemsToSync bool
	for _, itemDiff := range itemDiffs {
		// check if itemDiff is for a path to be excluded
		if matchesPathsToExclude(si.root, itemDiff.rootRelPath, si.exclude) {
			debugPrint(si.debug, fmt.Sprintf("syncDBwithFS | excluding: %s", itemDiff.rootRelPath))
			continue
		}

		switch itemDiff.diff {
		case localNewer:
			//addToDB
			debugPrint(si.debug, fmt.Sprintf("syncDBwithFS | local %s is newer", itemDiff.rootRelPath))
			itemDiff.remote.Content.SetText(itemDiff.local)
			itemsToPush = append(itemsToPush, itemDiff)
			itemsToSync = true
		case localMissing:
			// createLocal
			debugPrint(si.debug, fmt.Sprintf("syncDBwithFS | %s is missing", itemDiff.rootRelPath))
			itemsToPull = append(itemsToPull, itemDiff)
			itemsToSync = true
		case remoteNewer:
			// createLocal
			debugPrint(si.debug, fmt.Sprintf("syncDBwithFS | remote %s is newer", itemDiff.rootRelPath))
			itemsToPull = append(itemsToPull, itemDiff)
			itemsToSync = true
		}
	}

	// check items to sync
	if !itemsToSync {
		so.msg = fmt.Sprint(bold("nothing to do"))
		return
	}

	// addToDB
	if len(itemsToPush) > 0 {
		err = addToDB(si.db, si.session, itemsToPush, si.close)
		if err != nil {
			return
		}
		so.noPushed = len(itemsToPush)
	}

	res := make([]string, len(itemsToPush))
	strPushed := green("pushed")
	strPulled := green("pulled")

	for i, pushItem := range itemsToPush {
		line := fmt.Sprintf("%s | %s", bold(addDot(pushItem.rootRelPath)), strPushed)
		res[i] = line
	}

	// create local
	if err = createLocal(itemsToPull); err != nil {
		return
	}

	so.noPulled = len(itemsToPull)

	for _, pullItem := range itemsToPull {
		line := fmt.Sprintf("%s | %s\n", bold(addDot(pullItem.rootRelPath)), strPulled)
		res = append(res, line)
	}

	so.msg = fmt.Sprint(columnize.SimpleFormat(res))

	return so, err
}

type syncInput struct {
	db             *storm.DB
	session        *cache.Session
	twn            tagsWithNotes
	root           string
	paths, exclude []string
	debug          bool
	close          bool
}

type syncOutput struct {
	noPushed, noPulled int
	msg                string
}

func ensureTrailingPathSep(in string) string {
	if strings.HasSuffix(in, string(os.PathSeparator)) {
		return in
	}

	return in + string(os.PathSeparator)
}

func matchesPathsToExclude(root, path string, pathsToExclude []string) bool {
	for _, pte := range pathsToExclude {
		rootStrippedPath := stripHome(pte, root)
		// return match if Paths match exactly
		if rootStrippedPath == path {
			return true
		}
		// return match if pte is a parent of the path
		if strings.HasPrefix(path, ensureTrailingPathSep(rootStrippedPath)) {
			return true
		}
	}

	return false
}
