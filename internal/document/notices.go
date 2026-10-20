package document

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Every notice a read adds is built here, from a fixed sentence in brackets;
// only a file name, page numbers and the time limit are filled in. Nothing a
// document says reaches one.

func noticePassword(name string) string {
	return fmt.Sprintf("[%s is password-protected, so Hearth cannot read it. Ask the owner for a copy without the password.]", name)
}

func noticeNoPages(name string) string {
	return fmt.Sprintf("[%s has no pages Hearth can read.]", name)
}

// noticePictures names the picture pages. Without the app they could not be
// read at all; with one they were offered to it and not recognised.
func noticePictures(pages []int, app bool) string {
	if app {
		if len(pages) == 1 {
			return fmt.Sprintf("[Page %s is a picture with no text; it could not be recognised.]", pageList(pages))
		}
		return fmt.Sprintf("[Pages %s are pictures with no text; they could not be recognised.]", pageList(pages))
	}
	if len(pages) == 1 {
		return fmt.Sprintf("[Page %s is a picture with no text. Hearth isn't open, so it could not be read.]", pageList(pages))
	}
	return fmt.Sprintf("[Pages %s are pictures with no text. Hearth isn't open, so they could not be read.]", pageList(pages))
}

func noticeUnread(pages []int) string {
	if len(pages) == 1 {
		return fmt.Sprintf("[Page %s could not be read.]", pageList(pages))
	}
	return fmt.Sprintf("[Pages %s could not be read.]", pageList(pages))
}

// noticeTimeLimit says the read ran out of time at page at of total; at 0 it
// had not yet opened the file.
func noticeTimeLimit(limit time.Duration, at, total int) string {
	secs := int((limit + time.Second - 1) / time.Second)
	if at == 0 {
		return fmt.Sprintf("[Reading stopped after %d seconds, before any page was read.]", secs)
	}
	return fmt.Sprintf("[Reading stopped after %d seconds at page %d of %d.]", secs, at, total)
}

func noticeTooManyPages(from, to int) string {
	return fmt.Sprintf(`[Pages %d to %d were not read; call files.read with pages "%s".]`, from, to, PagesHint(from, to))
}

func noticeTextCap(at, to int) string {
	return fmt.Sprintf(`[Reading stopped at %d MB of text, after page %d. To read on, call files.read with pages "%s".]`, MaxText>>20, at, PagesHint(at+1, to))
}

// PagesHint is the range to pass as pages to read on from page from, at most
// MaxPages of it, ending no later than page to.
func PagesHint(from, to int) string {
	return fmt.Sprintf("%d-%d", from, min(to, from+MaxPages-1))
}

// pageList is page numbers, ascending, as the owner reads them: runs
// collapsed, "3 and 7", "1-3, 5 and 9".
func pageList(pages []int) string {
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
