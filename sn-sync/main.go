package snsync

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/asdine/storm/v3"
	"github.com/asdine/storm/v3/q"
	"github.com/fatih/color"
	"github.com/jonhadfield/gosn-v2/cache"
	"github.com/jonhadfield/gosn-v2/items"
)

const (
	// SNServerURL defines the default URL for making calls to syncDBwithFS with SN
	SNServerURL = "https://syncDBwithFS.standardnotes.org"
	// DotFilesTag defines the default tag that all SN Dotfiles will be prefixed with
	DotFilesTag = "sync"
	// DefaultPageSize defines the number of items to attempt to syncDBwithFS per request
	DefaultPageSize = 500
	// SpinnerCharSet defines the characters to use for the spinner shown when syncing
	SpinnerCharSet = 14
	// SpinnerDelay defines the number of milliseconds to wait between each character in the spinner
	SpinnerDelay = 100

	SNAppName = "sn-sync"
)

var (
	bold   = color.New(color.Bold).SprintFunc()
	red    = color.New(color.FgRed).SprintFunc()
	green  = color.New(color.FgGreen).SprintFunc()
	yellow = color.New(color.FgYellow).SprintFunc()
)

func getTagsWithNotes(db *storm.DB, session *cache.Session) (t tagsWithNotes, err error) {
	// validate session
	if !session.Valid() {
		err = errors.New("invalid session")
		return
	}

	var notesAndTags cache.Items

	if e := db.Select(q.In("ContentType", []string{"Note", "Tag", "SN|Component", "Extension"})).Find(&notesAndTags); e != nil {
		if e.Error() != "not found" {
			return
		}
	}

	var allItems items.Items
	allItems, err = notesAndTags.ToItems(session)
	if err != nil {
		return
	}

	var dotfileTags items.Tags

	var notes items.Notes

	r := regexp.MustCompile(fmt.Sprintf("%s.?.*", DotFilesTag))

	for _, item := range allItems {
		if item.GetContent() != nil && item.GetContentType() == "Tag" && r.MatchString(item.GetContent().(*items.TagContent).Title) {
			tt := item.(*items.Tag)
			dotfileTags = append(dotfileTags, *tt)
		}

		if item.GetContentType() == "Note" && item.GetContent() != nil {
			n := item.(*items.Note)
			notes = append(notes, *n)
		}
	}

	for _, dotfileTag := range dotfileTags {
		twn := tagWithNotes{
			tag: dotfileTag,
		}

		for _, note := range notes {
			if StringInSlice(note.GetUUID(), getItemNoteRefIds(dotfileTag.GetContent().References()), false) {
				twn.notes = append(twn.notes, note)
			}
		}

		t = append(t, twn)
	}

	return t, err
}

func getItemNoteRefIds(itemRefs items.ItemReferences) (refIds []string) {
	for _, ir := range itemRefs {
		if ir.ContentType == "Note" {
			refIds = append(refIds, ir.UUID)
		}
	}

	return refIds
}

type tagWithNotes struct {
	tag   items.Tag
	notes items.Notes
}

type tagsWithNotes []tagWithNotes

// GetNoteConfig defines the input for getting notes from SN
type GetNoteConfig struct {
	Session    cache.Session
	Filters    items.ItemFilters
	NoteTitles []string
	TagTitles  []string
	TagUUIDs   []string
	PageSize   int
	Debug      bool
}
