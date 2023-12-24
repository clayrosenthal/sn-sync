package snsync

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/fatih/set"
)

// preflight validates and tidies up the root directory and paths provided
func preflight(root string, in []string) (out []string, err error) {
	// check root is present
	if len(root) == 0 {
		err = errors.New("root undefined")
		return
	}

	// remove any duplicate paths
	in = dedupe(in)

	// handle shell expansion
	var v bool
	for _, inPath := range in {
		if strings.HasPrefix(inPath, "~") {
			inPath = strings.Replace(inPath, "~", root, 1)
			if v, err = pathValid(inPath); !v {
				return
			}
			out = append(out, inPath)
			continue
		}
		if !strings.HasPrefix(inPath, "/") {
			out = append(out, filepath.Join(root, inPath))
			if v, err = pathValid(inPath); !v {
				return
			}
			continue
		}
		if v, err = pathValid(inPath); !v {
			return
		}
		out = append(out, inPath)
	}

	return
}

func checkNoteTagConflicts(twn tagsWithNotes) error {
	// check for path conflict where tag and note overlap
	tagPaths := set.New(set.NonThreadSafe)
	notePaths := set.New(set.NonThreadSafe)

	for _, t := range twn {
		tagPath := t.tag.Content.GetTitle()
		tagPaths.Add(tagPath)
		// loop through tag related notes and generate a list
		// of all combinations to check for duplicates
		for _, n := range t.notes {
			var notePath string
			// if tag path is not root (DotFilesTag) then it's a sub tag/dir
			// so add tag path (plus period) to note title
			if tagPath != DotFilesTag {
				notePath = tagPath + "." + n.Content.GetTitle()
			} else {
				// otherwise, just add note title to DotFilesTag
				notePath = tagPath + n.Content.GetTitle()
			}

			notePaths.Add(notePath)
		}
	}

	inter := set.Intersection(tagPaths, notePaths)
	overlaps := make([]string, len(inter.List()))

	for c, i := range inter.List() {
		overlaps[c] = "- " + i.(string)
	}

	if inter.IsEmpty() {
		return nil
	}

	return fmt.Errorf("the following notes and tags are overlapping:\n%s", strings.Join(overlaps, "\n"))
}
