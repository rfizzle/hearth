// SubmitOnReturn is the rule for a box that is a message to someone: Return
// sends it and Shift-Return starts a new line. The composer and a setup
// sheet's opening question use it, the two places where what is typed goes to
// a person; every other editor keeps Return as a new line, because it is a
// field. It is a key-press handler on the box, so the field's own keyboard and
// VoiceOver behaviour is otherwise untouched.
// See docs/design.md#forms-sheets-and-settings

import SwiftUI

extension View {
    /// Calls `action` when Return is pressed in this view without Shift, and
    /// lets Shift-Return through as a new line.
    public func submitOnReturn(_ action: @escaping () -> Void) -> some View {
        onKeyPress(keys: [.return], phases: .down) { press in
            if press.modifiers.contains(.shift) { return .ignored }
            action()
            return .handled
        }
    }
}
