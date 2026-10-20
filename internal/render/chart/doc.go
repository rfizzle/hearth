// Package chart turns a line, bar or pie widget into a drawing of primitives
// and writes that drawing as SVG.
//
// Responsibility: Layout is the one place a chart's geometry is decided, in a
// fixed space of 600 by 320 units (a pie 600 by 360); SVG and the PDF drawing
// read the same Drawing, so the page and the paper show the same chart from the
// same numbers.
//
// Invariants the types do not hold:
//   - A layout is a pure function of the numbers and labels: no map iteration,
//     no time, no randomness, coordinates rounded to two decimals, labels in
//     the order given, hues by series order.
//   - SVG writes only numbers and escaped text; no attribute comes from input,
//     and ink and grid are currentColor so the page decides how they look.
//   - Layout takes a valid widget and still refuses one it cannot draw.
//
// See docs/design.md#colour.
package chart
