// NoteRow is one thing a bot remembers: the words, the day it was noted and,
// when the owner pinned it, a pin glyph in the tint. The context menu pins or
// unpins, edits in place and deletes; deleting asks first. Editing swaps the
// words for a text field that saves on Return and drops the change on Escape.
// The row decides nothing; each action calls back.
// See docs/model.md#knowledge

import HearthDesign
import HearthKit
import SwiftUI

/// A note in the inspector.
public struct NoteRow: View {
    let text: String
    let date: Date
    let isPinned: Bool
    let onPin: (Bool) -> Void
    let onEdit: (String) -> Void
    let onDelete: () -> Void

    @State private var isEditing: Bool
    @State private var draft: String
    @State private var isConfirming = false
    @FocusState private var isFocused: Bool

    public init(
        text: String, date: Date, isPinned: Bool,
        onPin: @escaping (Bool) -> Void, onEdit: @escaping (String) -> Void,
        onDelete: @escaping () -> Void
    ) {
        self.init(
            text: text, date: date, isPinned: isPinned, editing: false,
            onPin: onPin, onEdit: onEdit, onDelete: onDelete)
    }

    init(
        text: String, date: Date, isPinned: Bool, editing: Bool,
        onPin: @escaping (Bool) -> Void, onEdit: @escaping (String) -> Void,
        onDelete: @escaping () -> Void
    ) {
        self.text = text
        self.date = date
        self.isPinned = isPinned
        self.onPin = onPin
        self.onEdit = onEdit
        self.onDelete = onDelete
        _isEditing = State(initialValue: editing)
        _draft = State(initialValue: text)
    }

    public var body: some View {
        HStack(alignment: .firstTextBaseline, spacing: Theme.Spacing.sm) {
            VStack(alignment: .leading, spacing: Theme.Spacing.xs) {
                if isEditing {
                    TextField("Note", text: $draft, axis: .vertical)
                        .textFieldStyle(.roundedBorder)
                        .font(Theme.body)
                        .focused($isFocused)
                        .onSubmit(commit)
                        .onExitCommand(perform: cancel)
                        .accessibilityLabel("Edit note")
                } else {
                    Text(text)
                        .font(Theme.body)
                        .fixedSize(horizontal: false, vertical: true)
                }
                Text(date.formatted(.dateTime.month(.abbreviated).day()))
                    .font(Theme.caption)
                    .foregroundStyle(Theme.color(.textSecondary))
            }
            Spacer(minLength: 0)
            if isPinned {
                Image(systemName: "pin.fill")
                    .font(Theme.caption)
                    .foregroundStyle(.tint)
                    .accessibilityLabel("Pinned")
            }
        }
        .padding(.vertical, Theme.Spacing.xs)
        .contentShape(Rectangle())
        .contextMenu {
            Button(
                isPinned ? "Unpin" : "Pin", systemImage: isPinned ? "pin.slash" : "pin"
            ) { onPin(!isPinned) }
            Button("Edit", systemImage: "pencil", action: beginEditing)
            Divider()
            Button("Delete…", systemImage: "trash", role: .destructive) { isConfirming = true }
        }
        .confirmationDialog(
            "Make her forget this?", isPresented: $isConfirming, titleVisibility: .visible
        ) {
            Button("Delete", role: .destructive, action: onDelete)
        } message: {
            Text(text)
        }
        .accessibilityElement(children: isEditing ? .contain : .combine)
        .accessibilityValue(isPinned ? "Pinned" : "")
    }

    private func beginEditing() {
        draft = text
        isEditing = true
        isFocused = true
    }

    private func commit() {
        let words = draft.trimmingCharacters(in: .whitespacesAndNewlines)
        isEditing = false
        if !words.isEmpty, words != text { onEdit(words) }
    }

    private func cancel() {
        isEditing = false
        draft = text
    }
}

extension NoteRow {
    /// The states the gallery and previews show.
    public enum PreviewState: String, CaseIterable, Sendable {
        case plain
        case pinned
        case editing
        case long

        var text: String {
            switch self {
            case .long:
                "The owner wants every brief to open with the answer in one line, then the "
                    + "detail, and to name any number that came from a guess rather than a source."
            case .pinned: Fixtures.notes[0].text
            default: Fixtures.notes[1].text
            }
        }
    }

    /// A row showing `state`, for the gallery and previews.
    public init(previewState state: PreviewState) {
        self.init(
            text: state.text, date: Fixtures.notes[0].createdAt.date,
            isPinned: state == .pinned, editing: state == .editing,
            onPin: { _ in }, onEdit: { _ in }, onDelete: {})
    }
}

#Preview("Plain") { NoteRow(previewState: .plain).padding() }
#Preview("Pinned") { NoteRow(previewState: .pinned).padding() }
