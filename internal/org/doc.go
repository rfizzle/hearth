// Package org holds hearth's domain: the org, its bots, their grants, the
// conversations they talk in, the messages in them and the runs that answer.
//
// Responsibility: plain types and the pure rules over them. A new bot's
// default grants and its DM, what a set of grants allows, the folders a
// capability reaches, and the caps every run ends at.
//
// Invariants the types do not hold:
//   - The package imports nothing from hearth and nothing outside the
//     standard library, so every other package can depend on it.
//   - It does no I/O and mints no ids. A value built here has an empty id
//     until the store saves it.
//   - The numeric value of each enum is stored on disk, so a value is never
//     renumbered; a new one is appended.
//   - A capability with no grant is not granted. Nothing here widens access
//     the grants do not name.
package org
