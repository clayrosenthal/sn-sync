package sndotfiles

import (
	"fmt"
	"github.com/jonhadfield/gosn-v2"
	"github.com/jonhadfield/gosn-v2/cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"regexp"
	"testing"
)

func TestRemoveNoItems(t *testing.T) {
	err := removeFromDB(removeInput{session: testCacheSession, items: gosn.Items{}})
	assert.Error(t, err)
}

func TestRemoveItemsInvalidSession(t *testing.T) {
	tag := gosn.NewTag()
	tagContent := gosn.NewTagContent()
	tagContent.SetTitle("newTag")

	err := removeFromDB(removeInput{session: &cache.Session{
		Session:     nil,
		CacheDB:     nil,
		CacheDBPath: "",
	}, items: gosn.Items{&tag}})

	assert.Error(t, err)
}

func TestRemoveInvalidSession(t *testing.T) {
	home := getTemporaryHome()
	debugPrint(true, fmt.Sprintf("test | using temp home: %s", home))
	fwc := make(map[string]string)
	gitConfigPath := fmt.Sprintf("%s/.gitconfig", home)
	fwc[gitConfigPath] = "git config content"

	assert.NoError(t, createTemporaryFiles(fwc))

	ri := RemoveInput{
		Session: &cache.Session{},
		Home:    home,
		Paths:   []string{gitConfigPath},
		Debug:   true,
	}

	_, err := Remove(ri)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid")
}

func TestRemoveInvalidPath(t *testing.T) {
	ri := RemoveInput{
		Session: testCacheSession,
		Home:    getTemporaryHome(),
		Paths:   []string{"/invalid"},
		Debug:   true,
	}
	_, err := Remove(ri)
	assert.Error(t, err)
}

func TestRemoveNoPaths(t *testing.T) {
	ri := RemoveInput{
		Session: testCacheSession,
		Home:    getTemporaryHome(),
		Paths:   nil,
		Debug:   true,
	}
	_, err := Remove(ri)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "paths")
}

func TestRemoveTags(t *testing.T) {
	defer func() {
		if err := CleanUp(*testCacheSession); err != nil {
			fmt.Println("failed to wipe")
		}
	}()
	home := getTemporaryHome()
	debugPrint(true, fmt.Sprintf("test | using temp home: %s", home))

	fwc := make(map[string]string)
	gitConfigPath := fmt.Sprintf("%s/.gitconfig", home)
	fwc[gitConfigPath] = "git config content"
	applePath := fmt.Sprintf("%s/.fruit/apple", home)
	fwc[applePath] = "apple content"

	assert.NoError(t, createTemporaryFiles(fwc))
	// add items
	var err error
	ai := AddInput{Session: testCacheSession, Home: home, Paths: []string{gitConfigPath, applePath}}
	var ao AddOutput
	ao, err = Add(ai)
	assert.NoError(t, err)
	assert.Len(t, ao.PathsAdded, 2)
	assert.Len(t, ao.PathsExisting, 0)
	assert.Len(t, ao.PathsInvalid, 0)

	// removeFromDB single path
	ri := RemoveInput{
		Session: testCacheSession,
		Home:    home,
		Paths:   []string{gitConfigPath},
		Debug:   true,
	}

	var ro RemoveOutput
	ro, err = Remove(ri)
	assert.NoError(t, err)
	assert.Equal(t, 1, ro.NotesRemoved)
	assert.Equal(t, 0, ro.TagsRemoved)
	assert.Equal(t, 0, ro.NotTracked)
	assert.NotEmpty(t, ro.Msg)
	re := regexp.MustCompile("\\.gitconfig\\s+removed")
	assert.True(t, re.MatchString(ro.Msg))

}

func TestRemoveItems(t *testing.T) {
	defer func() {
		if err := CleanUp(*testCacheSession); err != nil {
			fmt.Println("failed to wipe")
		}
	}()
	home := getTemporaryHome()
	debugPrint(true, fmt.Sprintf("test | using temp home: %s", home))

	fwc := make(map[string]string)
	gitConfigPath := fmt.Sprintf("%s/.gitconfig", home)
	fwc[gitConfigPath] = "git config content"
	applePath := fmt.Sprintf("%s/.fruit/apple", home)
	fwc[applePath] = "apple content"
	yellowPath := fmt.Sprintf("%s/.fruit/banana/yellow", home)
	fwc[yellowPath] = "yellow content"
	premiumPath := fmt.Sprintf("%s/.cars/mercedes/a250/premium", home)
	fwc[premiumPath] = "premium content"

	assert.NoError(t, createTemporaryFiles(fwc))
	// add items
	ai := AddInput{Session: testCacheSession, Home: home, Paths: []string{gitConfigPath, applePath, yellowPath, premiumPath}}

	debugPrint(true, "Adding four paths")

	ao, err := Add(ai)
	assert.NoError(t, err)
	assert.Len(t, ao.PathsAdded, 4)
	assert.Len(t, ao.PathsExisting, 0)
	assert.Len(t, ao.PathsInvalid, 0)

	debugPrint(true, "removing ./gitconfig")

	// removeFromDB single path
	ri := RemoveInput{
		Session: testCacheSession,
		Home:    home,
		Paths:   []string{gitConfigPath},
		Debug:   true,
	}

	var ro RemoveOutput
	ro, err = Remove(ri)
	assert.NoError(t, err)
	assert.Equal(t, 1, ro.NotesRemoved)
	assert.Equal(t, 0, ro.TagsRemoved)
	assert.Equal(t, 0, ro.NotTracked)
	assert.NotEmpty(t, ro.Msg)
	re := regexp.MustCompile("\\.gitconfig\\s+removed")
	assert.True(t, re.MatchString(ro.Msg))

	// removeFromDB nested path with single item (with trailing slash)
	ri = RemoveInput{
		Session: testCacheSession,
		Home:    home,
		Paths:   []string{fmt.Sprintf("%s/.cars/", home)},
		Debug:   true,
	}

	debugPrint(true, "Removing \".cars/\"")
	ro, err = Remove(ri)
	assert.NoError(t, err)
	assert.Equal(t, 1, ro.NotesRemoved)
	assert.Equal(t, 3, ro.TagsRemoved)
	assert.Equal(t, 0, ro.NotTracked)
	assert.NotEmpty(t, ro.Msg)
	re = regexp.MustCompile("\\.cars/mercedes/a250/premium\\s+removed")
	assert.True(t, re.MatchString(ro.Msg))

	// get populated db
	si := cache.SyncInput{
		Session: testCacheSession,
		Close:   false,
	}

	var cso cache.SyncOutput
	cso, err = cache.Sync(si)
	require.NoError(t, err)

	var all tagsWithNotes
	all, err = getTagsWithNotes(cso.DB, testCacheSession)
	debugPrint(true, "after removing all .cars we have")
	for k, v := range all {
		debugPrint(true, fmt.Sprint(k, v))
	}
	assert.NoError(t, cso.DB.Close())

	// removeFromDB nested path with single item (without trailing slash)
	ri = RemoveInput{
		Session: testCacheSession,
		Home:    home,
		Paths:   []string{fmt.Sprintf("%s/.fruit", home)},
		Debug:   false,
	}

	ro, err = Remove(ri)
	assert.NoError(t, err)
	assert.Equal(t, 2, ro.NotesRemoved)
	assert.Equal(t, 3, ro.TagsRemoved)
	assert.Equal(t, 0, ro.NotTracked)
	assert.NotEmpty(t, ro.Msg)
	re = regexp.MustCompile("\\.fruit/apple\\s+removed")
	assert.True(t, re.MatchString(ro.Msg))
	re = regexp.MustCompile("\\.fruit/banana/yellow\\s+removed")
	assert.True(t, re.MatchString(ro.Msg))

	// ensure error with missing home
	ri = RemoveInput{
		Session: testCacheSession,
		Home:    "",
		Paths:   []string{fmt.Sprintf("%s/.fruit", home)},
		Debug:   false,
	}

	ro, err = Remove(ri)

	assert.Error(t, err)

	// ensure error with missing paths
	ri = RemoveInput{
		Session: testCacheSession,
		Home:    home,
		Paths:   []string{""},
		Debug:   true,
	}

	ro, err = Remove(ri)
	assert.Error(t, err)
}

func TestRemoveItemsRecursive(t *testing.T) {
	defer func() {
		if err := CleanUp(*testCacheSession); err != nil {
			fmt.Println("failed to wipe")
		}
	}()

	home := getTemporaryHome()
	debugPrint(true, fmt.Sprintf("test | using temp home: %s", home))

	fwc := make(map[string]string)
	gitConfigPath := fmt.Sprintf("%s/.gitconfig", home)
	fwc[gitConfigPath] = "git config content"
	applePath := fmt.Sprintf("%s/.fruit/apple", home)
	fwc[applePath] = "apple content"
	yellowPath := fmt.Sprintf("%s/.fruit/banana/yellow", home)
	fwc[yellowPath] = "yellow content"
	premiumPath := fmt.Sprintf("%s/.cars/mercedes/a250/premium", home)
	fwc[premiumPath] = "premium content"
	// path to recursively removeFromDB
	fruitPath := fmt.Sprintf("%s/.fruit", home)
	// try removing same path twice
	fruitPathDupe := fmt.Sprintf("%s/.fruit", home)

	assert.NoError(t, createTemporaryFiles(fwc))
	// add items
	ai := AddInput{Session: testCacheSession, Home: home, Paths: []string{gitConfigPath, applePath, yellowPath, premiumPath}}
	ao, err := Add(ai)
	assert.NoError(t, err)
	assert.Len(t, ao.PathsAdded, 4)
	assert.Len(t, ao.PathsExisting, 0)
	// try removing overlapping path and note in specified path

	ri := RemoveInput{
		Session: testCacheSession,
		Home:    home,
		Paths:   []string{yellowPath, fruitPath, fruitPathDupe},
		Debug:   true,
	}

	var ro RemoveOutput
	ro, err = Remove(ri)
	assert.NoError(t, err)
	assert.Equal(t, 2, ro.NotesRemoved)
	assert.Equal(t, 2, ro.TagsRemoved)
	assert.Equal(t, 0, ro.NotTracked)
}

func TestRemoveItemsRecursiveTwo(t *testing.T) {
	defer func() {
		if err := CleanUp(*testCacheSession); err != nil {
			fmt.Println("failed to wipe")
		}
	}()

	home := getTemporaryHome()
	debugPrint(true, fmt.Sprintf("test | using temp home: %s", home))

	fwc := make(map[string]string)
	gitConfigPath := fmt.Sprintf("%s/.gitconfig", home)
	fwc[gitConfigPath] = "git config content"
	greenPath := fmt.Sprintf("%s/.fruit/banana/green", home)
	fwc[greenPath] = "apple content"
	yellowPath := fmt.Sprintf("%s/.fruit/banana/yellow", home)
	fwc[yellowPath] = "yellow content"
	premiumPath := fmt.Sprintf("%s/.cars/mercedes/a250/premium", home)
	fwc[premiumPath] = "premium content"
	// path to recursively removeFromDB
	fruitPath := fmt.Sprintf("%s/.fruit", home)

	assert.NoError(t, createTemporaryFiles(fwc))
	// add items
	ai := AddInput{Session: testCacheSession, Home: home, Paths: []string{gitConfigPath, greenPath, yellowPath, premiumPath}}
	ao, err := Add(ai)
	assert.NoError(t, err)
	assert.Len(t, ao.PathsAdded, 4)
	assert.Len(t, ao.PathsExisting, 0)

	ri := RemoveInput{
		Session: testCacheSession,
		Home:    home,
		Paths:   []string{fruitPath},
		Debug:   true,
	}

	var ro RemoveOutput
	ro, err = Remove(ri)
	assert.NoError(t, err)
	assert.Equal(t, 2, ro.NotesRemoved)
	assert.Equal(t, 2, ro.TagsRemoved)
	assert.Equal(t, 0, ro.NotTracked)
}

func TestRemoveItemsRecursiveThree(t *testing.T) {
	defer func() {
		if err := CleanUp(*testCacheSession); err != nil {
			fmt.Println("failed to wipe")
		}
	}()

	home := getTemporaryHome()
	debugPrint(true, fmt.Sprintf("test | using temp home: %s", home))

	fwc := make(map[string]string)
	gitConfigPath := fmt.Sprintf("%s/.gitconfig", home)
	fwc[gitConfigPath] = "git config content"
	greenPath := fmt.Sprintf("%s/.fruit/banana/green", home)
	fwc[greenPath] = "apple content"
	yellowPath := fmt.Sprintf("%s/.fruit/banana/yellow", home)
	fwc[yellowPath] = "yellow content"
	premiumPath := fmt.Sprintf("%s/.cars/mercedes/a250/premium", home)
	fwc[premiumPath] = "premium content"
	lokiPath := fmt.Sprintf("%s/.dogs/labrador/loki", home)
	fwc[lokiPath] = "chicken please content"
	housePath := fmt.Sprintf("%s/.house/flat", home)
	fwc[housePath] = "flat description"
	// paths to recursively removeFromDB
	fruitPath := fmt.Sprintf("%s/.fruit/", home)
	labradorPath := fmt.Sprintf("%s/.dogs/labrador", home)

	assert.NoError(t, createTemporaryFiles(fwc))
	// add items
	ai := AddInput{Session: testCacheSession, Home: home, Paths: []string{gitConfigPath, greenPath, yellowPath, premiumPath, labradorPath}}

	ao, err := Add(ai)
	assert.NoError(t, err)
	assert.Len(t, ao.PathsAdded, 5)
	assert.Len(t, ao.PathsExisting, 0)

	ri := RemoveInput{
		Session: testCacheSession,
		Home:    home,
		Paths:   []string{fruitPath, labradorPath, housePath},
		Debug:   true,
	}

	var ro RemoveOutput
	ro, err = Remove(ri)

	assert.NoError(t, err)
	assert.Equal(t, 3, ro.NotesRemoved)
	assert.Equal(t, 4, ro.TagsRemoved)
	assert.Equal(t, 1, ro.NotTracked)
}

func TestRemoveAndCheckRemoved(t *testing.T) {
	defer func() {
		if err := CleanUp(*testCacheSession); err != nil {
			fmt.Println("failed to wipe")
		}
	}()

	home := getTemporaryHome()
	debugPrint(true, fmt.Sprintf("test | using temp home: %s", home))

	fwc := make(map[string]string)
	gitConfigPath := fmt.Sprintf("%s/.gitconfig", home)
	fwc[gitConfigPath] = "git configuration"

	assert.NoError(t, createTemporaryFiles(fwc))
	// add items
	ai := AddInput{Session: testCacheSession, Home: home, Paths: []string{gitConfigPath}}
	ao, err := Add(ai)
	assert.NoError(t, err)
	assert.Len(t, ao.PathsAdded, 1)
	assert.Len(t, ao.PathsExisting, 0)

	ri := RemoveInput{
		Session: testCacheSession,
		Home:    home,
		Paths:   []string{gitConfigPath},
		Debug:   true,
	}

	var ro RemoveOutput
	ro, err = Remove(ri)

	assert.NoError(t, err)
	assert.Equal(t, 1, ro.NotesRemoved)
	assert.Equal(t, 1, ro.TagsRemoved)
	assert.Equal(t, 0, ro.NotTracked)

	var cso cache.SyncOutput
	cso, err = cache.Sync(cache.SyncInput{
		Session: testCacheSession,
	})
	require.NoError(t, err)

	twn, _ := getTagsWithNotes(cso.DB, testCacheSession)
	assert.Len(t, twn, 0)
	assert.NoError(t, cso.DB.Close())
}

func TestRemoveAndCheckRemovedOne(t *testing.T) {
	defer func() {
		if err := CleanUp(*testCacheSession); err != nil {
			fmt.Println("failed to wipe")
		}
	}()

	home := getTemporaryHome()
	debugPrint(true, fmt.Sprintf("test | using temp home: %s", home))

	fwc := make(map[string]string)
	gitConfigPath := fmt.Sprintf("%s/.gitconfig", home)
	fwc[gitConfigPath] = "git configuration"
	awsConfigPath := fmt.Sprintf("%s/.aws/config", home)
	fwc[awsConfigPath] = "aws config"
	acmeConfigPath := fmt.Sprintf("%s/.acme/config", home)
	fwc[acmeConfigPath] = "acme config"
	assert.NoError(t, createTemporaryFiles(fwc))
	// add items
	ai := AddInput{Session: testCacheSession, Home: home, Paths: []string{gitConfigPath, awsConfigPath, acmeConfigPath}}
	ao, err := Add(ai)
	assert.NoError(t, err)
	// dotfiles tag, .gitconfig, and acmeConfig should exist
	assert.Len(t, ao.PathsAdded, 3)
	assert.Len(t, ao.PathsExisting, 0)

	ri := RemoveInput{
		Session: testCacheSession,
		Home:    home,
		Paths:   []string{gitConfigPath, acmeConfigPath},
		Debug:   true,
	}

	var ro RemoveOutput
	ro, err = Remove(ri)

	assert.NoError(t, err)
	assert.Equal(t, 2, ro.NotesRemoved)
	assert.Equal(t, 1, ro.TagsRemoved)
	assert.Equal(t, 0, ro.NotTracked)
	var cso cache.SyncOutput
	cso, err = cache.Sync(cache.SyncInput{
		Session: testCacheSession,
	})
	require.NoError(t, err)

	twn, _ := getTagsWithNotes(cso.DB, testCacheSession)
	// dotfiles tag and .gitconfig note should exist
	assert.Len(t, twn, 2)
	assert.NoError(t, cso.DB.Close())
}
