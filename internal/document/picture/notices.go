package picture

import (
	"fmt"
	"strconv"
	"strings"
)

// Unread names the picture pages that were not read. Without the app they
// could not be read at all; with one they were offered and not recognised.
func Unread(pages []int, app bool) string {
	if app {
		return ofPages(pages, "[Page %s is a picture with no text; it could not be recognised.]",
			"[Pages %s are pictures with no text; they could not be recognised.]")
	}
	return ofPages(pages, "[Page %s is a picture with no text. Hearth isn't open, so it could not be read.]",
		"[Pages %s are pictures with no text. Hearth isn't open, so they could not be read.]")
}

// Seen names the picture pages whose text the app recognised by looking at
// them.
func Seen(pages []int) string {
	return ofPages(pages, "[Page %s is a picture; its text was recognised by Hearth and may have mistakes.]",
		"[Pages %s are pictures; their text was recognised by Hearth and may have mistakes.]")
}

// TextLayer names the pages whose text the core could not make out and the
// app's PDF reader could.
func TextLayer(pages []int) string {
	return ofPages(pages, "[Page %s had text Hearth's own reader could not make out; the Mac's PDF reader read it.]",
		"[Pages %s had text Hearth's own reader could not make out; the Mac's PDF reader read it.]")
}

// ofPages is one, or many when there is more than one page, with the pages
// filled in as PageList writes them.
func ofPages(pages []int, one, many string) string {
	if len(pages) == 1 {
		return fmt.Sprintf(one, PageList(pages))
	}
	return fmt.Sprintf(many, PageList(pages))
}

// Image says what became of the text in the image called name: answered is
// whether the app answered, text whether it found any. Without the app
// nothing could look.
func Image(name string, app, answered, text bool) string {
	switch {
	case !app:
		return fmt.Sprintf("[%s is a picture. Hearth isn't open, so its text could not be read.]", name)
	case !answered:
		return fmt.Sprintf("[%s is a picture; its text could not be recognised.]", name)
	case !text:
		return fmt.Sprintf("[%s is a picture; Hearth found no text in it.]", name)
	}
	return fmt.Sprintf("[%s is a picture; its text was recognised by Hearth and may have mistakes.]", name)
}

// PageList is page numbers, ascending, as the owner reads them: runs
// collapsed, "3 and 7", "1-3, 5 and 9".
func PageList(pages []int) string {
	var parts []string
	for i := 0; i < len(pages); {
		j := i
		for j+1 < len(pages) && pages[j+1] == pages[j]+1 {
			j++
		}
		if j > i {
			parts = append(parts, strconv.Itoa(pages[i])+"-"+strconv.Itoa(pages[j]))
		} else {
			parts = append(parts, strconv.Itoa(pages[i]))
		}
		i = j + 1
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return strings.Join(parts[:len(parts)-1], ", ") + " and " + parts[len(parts)-1]
}
