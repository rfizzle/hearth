// Package html renders a shaped deliverable as one self-contained web page in
// Hearth's look: the title, then each section as a grid of cards, one card a
// widget, charts as inline SVG and prose from the widget's Markdown.
//
// Responsibility: Render turns a valid deliverable.Spec into the bytes of the
// page. The shell and the ten widgets are html/template files under
// templates/; their class names are Tailwind's, and deliverable.css, the
// stylesheet the page inlines, is built from them offline (make tailwind) and
// committed. Dark mode is the stylesheet's variables under
// prefers-color-scheme, and print sets the page's orientation and margins.
//
// Invariants the types do not hold:
//   - The spec is model output and the page is a file the owner may share, so
//     the page is inert: no script, no external resource, no event attribute,
//     a content-security-policy that forbids scripts, frames, images and
//     fetches, and links only to http, https and mailto addresses.
//   - Text reaches the page by one escaping path, html/template: a {{.}} action
//     in a template, or template.HTMLEscapeString in the Markdown walker. The
//     only values the templates take as trusted are the committed stylesheet,
//     the walker's output and chart.SVG's, each converted in one place in
//     render.go.
//   - The Markdown walker writes only the elements a deliverable allows, from
//     constant strings, and drops raw HTML and images even though validation
//     refuses them first.
//   - The same Spec gives the same bytes: no clock, no host, no map ranged;
//     the only time on the page is a bound value's as_of, and every number
//     goes through deliverable.Format, as the chart's do.
//   - Every class the page uses is written out under templates/, or the build
//     leaves it out of the stylesheet.
//
// See docs/design.md#colour.
package html
