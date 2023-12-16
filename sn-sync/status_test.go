package snsync

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/jonhadfield/gosn-v2"

	"github.com/stretchr/testify/assert"
)

func TestStatusEmptyTWN(t *testing.T) {
	home := getTemporaryHome()
	_, msg, _ := status(tagsWithNotes{}, home, []string{}, true)
	assert.Equal(t, "no sync being tracked", msg)
}

func testStatusSetup() (twn tagsWithNotes) {
	syncTag := createTag("sync")
	gitconfigNote := createNote(".gitconfig", "git config content")
	syncTagWithNote := tagWithNotes{tag: syncTag, notes: gosn.Notes{gitconfigNote}}

	fruitTag := createTag("sync.fruit")
	fruitBananaTag := createTag("sync.fruit.banana")
	appleNote := createNote("apple", "apple content")
	lemonNote := createNote("lemon", "lemon content")
	grapeNote := createNote("grape", "grape content")
	fruitTagWithNotes := tagWithNotes{tag: fruitTag, notes: gosn.Notes{appleNote, lemonNote, grapeNote}}

	yellowNote := createNote("yellow", "yellow content")
	fruitBananaTagWithNotes := tagWithNotes{tag: fruitBananaTag, notes: gosn.Notes{yellowNote}}

	premiumNote := createNote("premium", "premium content")
	carsMercedesA250Tag := createTag("sync.cars.mercedes.a250")
	carsMercedesA250TagWithNotes := tagWithNotes{tag: carsMercedesA250Tag, notes: gosn.Notes{premiumNote}}

	twn = tagsWithNotes{syncTagWithNote, fruitTagWithNotes, fruitBananaTagWithNotes, carsMercedesA250TagWithNotes}
	return
}

func TestStatus(t *testing.T) {
	home := getTemporaryHome()
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

	twn := testStatusSetup()

	var diffs []ItemDiff
	var err error

	diffs, _, err = status(twn, home, []string{gitConfigPath, applePath, yellowPath, premiumPath}, true)
	assert.NoError(t, err)
	assert.Len(t, diffs, 4)
	var pDiff int
	for _, d := range diffs {
		switch d.path {
		case gitConfigPath:
			assert.Equal(t, ".gitconfig", d.noteTitle)
			assert.Equal(t, gitConfigPath, d.path)
			assert.Equal(t, identical, d.diff)
			pDiff++
		case applePath:
			assert.Equal(t, "apple", d.noteTitle)
			assert.Equal(t, applePath, d.path)
			assert.Equal(t, identical, d.diff)
			pDiff++
		case yellowPath:
			assert.Equal(t, "yellow", d.noteTitle)
			assert.Equal(t, yellowPath, d.path)
			assert.Equal(t, identical, d.diff)
			pDiff++
		case premiumPath:
			assert.Equal(t, "premium", d.noteTitle)
			assert.Equal(t, premiumPath, d.path)
			assert.Equal(t, identical, d.diff)
			pDiff++
		}
	}
	assert.Equal(t, 4, pDiff)
}

func TestStatus1(t *testing.T) {
	home := getTemporaryHome()

	fwc := make(map[string]string)
	gitConfigPath := fmt.Sprintf("%s/.gitconfig", home)
	fwc[gitConfigPath] = "git config content"
	awsConfig := fmt.Sprintf("%s/.aws/config", home)
	fwc[awsConfig] = "aws config content"

	assert.NoError(t, createTemporaryFiles(fwc))

	syncTag := createTag("sync")
	gitconfigNote := createNote(".gitconfig", "git config content")
	syncTagWithNote := tagWithNotes{tag: syncTag, notes: gosn.Notes{gitconfigNote}}

	awsTag := createTag("sync.aws")
	awsConfigNote := createNote("config", "aws config content")
	awsTagWithNotes := tagWithNotes{tag: awsTag, notes: gosn.Notes{awsConfigNote}}

	twn := tagsWithNotes{syncTagWithNote, awsTagWithNotes}

	diffs, _, err := status(twn, home, []string{gitConfigPath}, true)
	assert.NoError(t, err)
	assert.Len(t, diffs, 1)
	assert.Equal(t, ".gitconfig", diffs[0].noteTitle)
	assert.Equal(t, gitConfigPath, diffs[0].path)
	assert.Equal(t, identical, diffs[0].diff)
}

func TestStatus2(t *testing.T) {

	home := getTemporaryHome()

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

	syncTag := createTag("sync")
	gitconfigNote := createNote(".gitconfig", "git config content")
	syncTagWithNote := tagWithNotes{tag: syncTag, notes: gosn.Notes{gitconfigNote}}

	fruitTag := createTag("sync.fruit")
	fruitBananaTag := createTag("sync.fruit.banana")
	appleNote := createNote("apple", "apple content")
	fruitTagWithNotes := tagWithNotes{tag: fruitTag, notes: gosn.Notes{appleNote}}

	yellowNote := createNote("yellow", "yellow content")
	fruitBananaTagWithNotes := tagWithNotes{tag: fruitBananaTag, notes: gosn.Notes{yellowNote}}

	premiumNote := createNote("premium", "premium content")
	carsMercedesA250Tag := createTag("sync.cars.mercedes.a250")
	carsMercedesA250TagWithNotes := tagWithNotes{tag: carsMercedesA250Tag, notes: gosn.Notes{premiumNote}}

	twn := tagsWithNotes{syncTagWithNote, fruitTagWithNotes, fruitBananaTagWithNotes, carsMercedesA250TagWithNotes}

	var err error

	// delete apple so that a local item is missing
	err = os.Remove(applePath)
	assert.NoError(t, err)

	// update yellow content
	// wait so that update time comparison doesn't fail due to formats
	time.Sleep(1 * time.Second)
	d1 := []byte("new yellow content")
	assert.NoError(t, ioutil.WriteFile(yellowPath, d1, 0644))

	// create untracked file
	d1 = []byte("green content")
	greenPath := fmt.Sprintf("%s/.fruit/banana/green", home)
	assert.NoError(t, ioutil.WriteFile(greenPath, d1, 0644))
	// pause so that remote updated time newer
	time.Sleep(1 * time.Second)
	// update premium remote to trigger remote newer condition
	newPremiumNote := createNote("premium", "new content")
	newCarsMercedesA250TagWithNotes := tagWithNotes{tag: carsMercedesA250Tag, notes: gosn.Notes{newPremiumNote}}
	twn = tagsWithNotes{syncTagWithNote, fruitTagWithNotes, fruitBananaTagWithNotes, newCarsMercedesA250TagWithNotes}

	var diffs []ItemDiff

	diffs, _, err = status(twn, home, []string{fmt.Sprintf("%s/.fruit", home), fmt.Sprintf("%s/.cars", home)}, true)
	assert.NoError(t, err)
	assert.Len(t, diffs, 4)
	var pDiff int
	for _, d := range diffs {
		switch d.path {
		case gitConfigPath:
			assert.Equal(t, ".gitconfig", d.noteTitle)
			assert.Equal(t, gitConfigPath, d.path)
			assert.Equal(t, identical, d.diff)
			pDiff++
		case applePath:
			assert.Equal(t, "apple", d.noteTitle)
			assert.Equal(t, applePath, d.path)
			assert.Equal(t, localMissing, d.diff)
			pDiff++
		case yellowPath:
			assert.Equal(t, "yellow", d.noteTitle)
			assert.Equal(t, yellowPath, d.path)
			assert.Equal(t, localNewer, d.diff)
			pDiff++
		case greenPath:
			assert.Equal(t, greenPath, d.path)
			assert.Equal(t, untracked, d.diff)
			pDiff++
		case premiumPath:
			assert.Equal(t, "premium", d.noteTitle)
			assert.Equal(t, premiumPath, d.path)
			assert.Equal(t, remoteNewer, d.diff)
			pDiff++
		}
	}
	assert.Equal(t, 4, pDiff)
}
