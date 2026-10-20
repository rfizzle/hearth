// Package fonts holds the typefaces a rendered PDF is set in: Source Sans 3
// (regular, bold, italic) for text and Source Code Pro (regular) for code,
// embedded with their licence.
//
// Responsibility: hand the renderer each face's bytes under the family and
// style it registers them as. The files are Adobe's releases, unchanged;
// SOURCES.md records where each came from and its sum, and OFL.txt is the
// licence they are distributed under, embedded beside them.
//
// Invariants the types do not hold:
//   - A face is read from the binary only. Nothing names a font file on
//     disk, so no document can make the renderer load one.
//   - Faces returns a fresh copy of each file, so a renderer that writes
//     into the bytes cannot change the next render.
//
// It imports the standard library only. D39 in docs/decisions.md chose the
// families.
package fonts
