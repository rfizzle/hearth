package sheet

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/rfizzle/hearth/internal/document/office"
	"github.com/xuri/excelize/v2"
)

// The caps of one read.
const (
	// MaxSheets is how many sheets one read returns when none is chosen.
	MaxSheets = 20
	// MaxCells is how many cells of one sheet are shown: the rows kept times
	// the columns in use among them.
	MaxCells = office.MaxCells
	// MaxCellRunes is the longest a cell's text is before it is cut.
	MaxCellRunes = 500
	// MaxEntries is how many entries the zip may hold.
	MaxEntries = 10_000
	// MaxEntry is the most one entry may hold once decompressed, by its
	// header and by the bytes it really yields.
	MaxEntry = 64 << 20
	// MaxTotal is the most all the entries may hold once decompressed.
	MaxTotal = 256 << 20
)

var (
	// ErrNotWorkbook is a zip that has no workbook part.
	ErrNotWorkbook = errors.New("the file is not an Excel workbook")
	// ErrEncrypted is the container that holds a password-protected workbook
	// or a workbook in the old binary format.
	ErrEncrypted = errors.New("the file is protected by a password or in an old Excel format")
	// ErrLimit is a workbook over one of the zip caps.
	ErrLimit = errors.New("the workbook is over a size limit")
	// ErrDamaged is a workbook the library cannot open, or panics on.
	ErrDamaged = errors.New("the workbook is damaged or not supported")
)

// NoSheetError is a sheet that was asked for and is not in the workbook.
type NoSheetError struct{ Names []string }

func (e *NoSheetError) Error() string { return "the workbook has no such sheet" }

// Sheet is one sheet read.
type Sheet struct {
	Name string
	// Rows and Cols are what the sheet holds: its rows, and the most columns
	// any row uses. When Exact is false the count stopped at the deadline and
	// they are what had been counted.
	Rows, Cols int
	Exact      bool
	// Shown is how many cells the table holds; Cut is whether it is fewer
	// than the sheet has.
	Shown int
	Cut   bool
	// CellsCut is whether a cell was longer than MaxCellRunes and was cut.
	CellsCut bool
	// Unread is a sheet the library could not read.
	Unread bool
	// Table is the Markdown table, row 1 as its header; empty for a sheet
	// with no cells.
	Table string
}

// Result is a workbook read.
type Result struct {
	// Names are all the sheets in the workbook, in order.
	Names []string
	// Sheets are the sheets read, in order.
	Sheets []Sheet
	// Left are the names of sheets past MaxSheets, which were not read.
	Left []string
	// Stopped is the 1-based position of the sheet the deadline reached, 0
	// when it did not; the sheets before it are in Sheets. It is also 1 when
	// the deadline came before the workbook was open.
	Stopped int
}

// oleMagic begins the container of an encrypted or old-format workbook.
var oleMagic = []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}

// slots bounds the reads that can be running, or stuck, at once.
var (
	slotsMu sync.Mutex
	slots   = make(chan struct{}, 2)
)

// UseSlots replaces the pool with a fresh one of n slots and returns a
// function that puts the old one back, for a test that leaves a read stuck.
func UseSlots(n int) (restore func()) {
	slotsMu.Lock()
	defer slotsMu.Unlock()
	old := slots
	slots = make(chan struct{}, n)
	return func() {
		slotsMu.Lock()
		defer slotsMu.Unlock()
		slots = old
	}
}

// state is what the worker has found so far; the caller reads it under mu
// when the deadline ends the wait, while the worker may still be running.
type state struct {
	mu  sync.Mutex
	res Result
}

// Read reads the workbook in f, size bytes long. A non-empty sel chooses one
// sheet, by name or by its number from 1 (a name wins over a number); empty
// reads the first MaxSheets. When ctx ends first it returns ctx's error with
// the sheets finished so far in the Result.
func Read(ctx context.Context, f io.ReaderAt, size int64, sel string) (Result, error) {
	slotsMu.Lock()
	pool := slots
	slotsMu.Unlock()
	select {
	case pool <- struct{}{}:
	case <-ctx.Done():
		return Result{Stopped: 1}, ctx.Err()
	}
	st := &state{}
	done := make(chan error, 1)
	go func() {
		err := ErrDamaged // what a panic outside a sheet leaves
		defer func() {
			_ = recover()
			<-pool
			done <- err
		}()
		err = st.read(ctx, f, size, sel)
	}()
	select {
	case err := <-done:
		return st.snapshot(), err
	case <-ctx.Done():
		r := st.snapshot()
		if r.Stopped == 0 {
			r.Stopped = len(r.Sheets) + 1
		}
		return r, ctx.Err()
	}
}

func (st *state) snapshot() Result {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.res
}

// openOptions are the library's limits: the caps of this package, with the
// threshold above which it spools a part to a temporary file raised to the
// total so that it never does, and the temporary folder set to a path that
// cannot hold a file as a second guard.
func openOptions() excelize.Options {
	return excelize.Options{
		UnzipSizeLimit:    MaxTotal,
		UnzipXMLSizeLimit: MaxTotal,
		TmpDir:            os.DevNull,
	}
}

func (st *state) read(ctx context.Context, f io.ReaderAt, size int64, sel string) error {
	if err := check(ctx, f, size); err != nil {
		return err
	}
	book, err := excelize.OpenReader(io.NewSectionReader(f, 0, size), openOptions())
	if err != nil {
		return ErrDamaged
	}
	defer book.Close()
	names := book.GetSheetList()
	st.mu.Lock()
	st.res.Names = names
	st.mu.Unlock()

	todo := names
	switch {
	case sel != "":
		i := pick(names, sel)
		if i < 0 {
			return &NoSheetError{Names: names}
		}
		todo = names[i : i+1]
	case len(names) > MaxSheets:
		todo = names[:MaxSheets]
		st.mu.Lock()
		st.res.Left = append([]string(nil), names[MaxSheets:]...)
		st.mu.Unlock()
	}
	for i, name := range todo {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		s, err := readSheet(ctx, book, name)
		if err != nil { // the deadline reached this sheet
			st.mu.Lock()
			st.res.Stopped = i + 1
			st.mu.Unlock()
			return err
		}
		st.mu.Lock()
		st.res.Sheets = append(st.res.Sheets, s)
		st.mu.Unlock()
	}
	return nil
}

// pick is the index of the sheet sel names: an exact name, then a name that
// differs only in case, then a number from 1. It is -1 for none.
func pick(names []string, sel string) int {
	for i, n := range names {
		if n == sel {
			return i
		}
	}
	for i, n := range names {
		if strings.EqualFold(n, sel) {
			return i
		}
	}
	if n, err := strconv.Atoi(strings.TrimSpace(sel)); err == nil && n >= 1 && n <= len(names) {
		return n - 1
	}
	return -1
}

// check walks the zip before the library does: the entry count, each
// entry's claimed size, the total, then every entry decompressed once into
// nothing against the same numbers, so a header that understates its entry
// is caught here and not by the library's allocation.
func check(ctx context.Context, f io.ReaderAt, size int64) error {
	var head [8]byte
	if _, err := f.ReadAt(head[:], 0); err != nil {
		return ErrDamaged
	}
	if bytes.Equal(head[:], oleMagic) {
		return ErrEncrypted
	}
	zr, err := zip.NewReader(io.NewSectionReader(f, 0, size), size)
	if err != nil {
		return ErrDamaged
	}
	if len(zr.File) > MaxEntries {
		return ErrLimit
	}
	var claimed uint64
	book := false
	for _, e := range zr.File {
		if e.UncompressedSize64 > MaxEntry {
			return ErrLimit
		}
		if claimed += e.UncompressedSize64; claimed > MaxTotal {
			return ErrLimit
		}
		if strings.EqualFold(strings.ReplaceAll(e.Name, `\`, "/"), "xl/workbook.xml") {
			book = true
		}
	}
	if !book {
		return ErrNotWorkbook
	}
	var total int64
	for _, e := range zr.File {
		if err := ctx.Err(); err != nil {
			return err
		}
		rc, err := e.Open()
		if err != nil {
			return ErrDamaged
		}
		n, err := io.Copy(io.Discard, io.LimitReader(rc, MaxEntry+1))
		rc.Close()
		switch {
		case err != nil:
			return ErrDamaged
		case n > MaxEntry:
			return ErrLimit
		case uint64(n) != e.UncompressedSize64:
			return ErrDamaged // the header lied
		}
		if total += n; total > MaxTotal {
			return ErrLimit
		}
	}
	return nil
}

// readSheet reads one sheet. A panic or a library error leaves it Unread; an
// error return is only the deadline.
func readSheet(ctx context.Context, book *excelize.File, name string) (s Sheet, err error) {
	s = Sheet{Name: name, Exact: true}
	defer func() {
		if recover() != nil {
			s, err = Sheet{Name: name, Unread: true}, nil
		}
	}()
	rows, rerr := book.Rows(name)
	if rerr != nil {
		return Sheet{Name: name, Unread: true}, nil
	}
	defer rows.Close()

	var kept [][]string
	var keptCols int
	for n := 0; rows.Next(); n++ {
		if n%64 == 0 && ctx.Err() != nil {
			if !s.Cut { // still collecting: the sheet is lost
				return s, ctx.Err()
			}
			s.Exact = false // counting the rest: keep what was counted
			break
		}
		cols, cerr := rows.Columns()
		if cerr != nil {
			return Sheet{Name: name, Unread: true}, nil
		}
		for len(cols) > 0 && cols[len(cols)-1] == "" {
			cols = cols[:len(cols)-1]
		}
		s.Rows++
		s.Cols = max(s.Cols, len(cols))
		if s.Cut {
			continue
		}
		if (len(kept)+1)*max(keptCols, len(cols)) > MaxCells {
			s.Cut = true
			continue
		}
		cells := make([]string, len(cols))
		for i, c := range cols {
			var cut bool
			cells[i], cut = cell(c)
			s.CellsCut = s.CellsCut || cut
		}
		kept = append(kept, cells)
		keptCols = max(keptCols, len(cols))
	}
	if rows.Error() != nil {
		return Sheet{Name: name, Unread: true}, nil
	}
	s.Shown = len(kept) * keptCols
	s.Table = office.Table(kept)
	return s, nil
}

// cell is a cell's text cut at MaxCellRunes; the table writer makes it one
// line and escapes the pipe.
func cell(s string) (string, bool) {
	if utf8.RuneCountInString(s) > MaxCellRunes {
		return string([]rune(s)[:MaxCellRunes]) + "…", true
	}
	return s, false
}
