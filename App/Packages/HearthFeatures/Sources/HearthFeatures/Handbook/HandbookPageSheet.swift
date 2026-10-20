// HandbookPageSheet writes one handbook page: a title, who reads it (two
// chips, Everyone and the bot by name, one chosen), the Markdown in a plain
// editor on a card with no preview, and under it the core's refusal if it
// gave one. The meter sits at the bottom, with Cancel and the one prominent
// Save. The sheet is a function of its drafts and the store; it reports a
// saved page or a cancel to its caller.
// See docs/design.md#forms-sheets-and-settings

import HearthDesign
import HearthKit
import HearthUI
import SwiftUI

/// The sheet that adds or edits one handbook page.
public struct HandbookPageSheet: View {
    let store: HandbookStore
    let page: Hearth_V1_HandbookPage?
    let onDone: () -> Void

    @State private var title: String
    @State private var markdown: String
    @State private var forEveryone: Bool

    /// A sheet for `page`, or for a new page when nil. `onDone` runs after a
    /// Save the core accepted and after Cancel.
    public init(
        store: HandbookStore, page: Hearth_V1_HandbookPage? = nil, onDone: @escaping () -> Void
    ) {
        self.store = store
        self.page = page
        self.onDone = onDone
        _title = State(initialValue: page?.title ?? "")
        _markdown = State(initialValue: page?.markdown ?? "")
        _forEveryone = State(initialValue: page?.scope.everyone ?? true)
    }

    public var body: some View {
        VStack(alignment: .leading, spacing: Theme.Spacing.lg) {
            Text(page == nil ? "New page" : "Edit page").font(Theme.title)
            TextField("Title", text: $title)
                .textFieldStyle(.roundedBorder)
                .font(Theme.body)
            scopeChips
            editor
            if let refusal = store.saveError {
                Label(refusal, systemImage: "exclamationmark.triangle")
                    .font(Theme.callout)
                    .foregroundStyle(Theme.color(.toneNegative))
            }
            ContextMeter(used: store.meter.usedBytes, cap: store.meter.capBytes)
            buttons
        }
        .padding(Theme.Spacing.xl)
        .frame(width: Theme.Size.sheetWidth)
        .presentationSizing(.form)
        .onAppear { store.clearSaveError() }
        .task { if !store.loaded { await store.load() } }
    }

    private var scopeChips: some View {
        HStack(spacing: Theme.Spacing.sm) {
            Text("Read by").font(Theme.callout).foregroundStyle(Theme.color(.textSecondary))
            chip("Everyone", isSelected: forEveryone) { forEveryone = true }
            chip(store.botName, isSelected: !forEveryone) { forEveryone = false }
        }
    }

    private func chip(_ text: String, isSelected: Bool, action: @escaping () -> Void) -> some View {
        Button(action: action) {
            Pill(text)
                .overlay { if isSelected { Capsule().strokeBorder(.tint, lineWidth: 2) } }
        }
        .buttonStyle(.plain)
        .accessibilityAddTraits(isSelected ? .isSelected : [])
    }

    private var editor: some View {
        CardSurface {
            TextEditor(text: $markdown)
                .font(Theme.monospaced)
                .scrollContentBackground(.hidden)
                .frame(minHeight: 220)
                .accessibilityLabel("Page text, in Markdown")
        }
    }

    private var buttons: some View {
        HStack {
            Button("Cancel") { onDone() }
                .keyboardShortcut(.cancelAction)
            Spacer()
            Button("Save") { Task { await save() } }
                .buttonStyle(.borderedProminent)
                .keyboardShortcut(.defaultAction)
                .disabled(store.isSaving)
        }
    }

    private func save() async {
        let scope = Hearth_V1_Scope.with {
            if forEveryone { $0.everyone = true } else { $0.botID = store.botID }
        }
        if await store.put(id: page?.id ?? "", title: title, markdown: markdown, scope: scope) {
            onDone()
        }
    }
}
