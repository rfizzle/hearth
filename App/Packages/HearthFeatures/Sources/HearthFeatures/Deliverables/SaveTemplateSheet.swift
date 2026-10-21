// SaveTemplateSheet keeps a deliverable as a template: a name, one sentence on
// when a bot should use it, and the id the name makes, shown so the owner sees
// the file it will be. Save turns the deliverable's spec file into a template
// (every section allows the kinds it holds; the widgets stay as the sample)
// and sends it; the core's refusal shows under the form in its words. The
// sheet is a function of its drafts and the store.
// See docs/design.md#forms-sheets-and-settings, docs/decisions.md (D43)

import HearthDesign
import HearthUI
import SwiftUI

/// The sheet that saves one deliverable as a template.
public struct SaveTemplateSheet: View {
    let store: TemplatesStore
    let specPath: String
    let onDone: () -> Void

    @State private var name: String
    @State private var when = ""
    @State private var unreadable = false

    /// A sheet for the spec file at `specPath`, its name starting as `title`.
    /// `onDone` runs after a save the core accepted and after Cancel.
    public init(
        store: TemplatesStore, specPath: String, title: String, onDone: @escaping () -> Void
    ) {
        self.store = store
        self.specPath = specPath
        self.onDone = onDone
        _name = State(initialValue: title)
    }

    private var id: String { TemplateDocument.slug(name) }

    public var body: some View {
        VStack(alignment: .leading, spacing: Theme.Spacing.lg) {
            Text("Save as template").font(Theme.title)
            Text("Your team will be offered it the next time someone writes a deliverable.")
                .font(Theme.callout)
                .foregroundStyle(Theme.color(.textSecondary))
            TextField("Name", text: $name)
                .textFieldStyle(.roundedBorder)
                .font(Theme.body)
            TextField("When to use it, in one sentence", text: $when, axis: .vertical)
                .textFieldStyle(.roundedBorder)
                .font(Theme.body)
                .lineLimit(2...3)
            Text(id.isEmpty ? "Give it a name to make its id." : "Saved as \(id).deliverable.json")
                .font(Theme.caption)
                .foregroundStyle(Theme.color(.textSecondary))
            if let refusal = unreadable ? "That file can't be read." : store.error {
                Label(refusal, systemImage: "exclamationmark.triangle")
                    .font(Theme.callout)
                    .foregroundStyle(Theme.color(.toneNegative))
            }
            HStack {
                Button("Cancel") { onDone() }
                    .keyboardShortcut(.cancelAction)
                Spacer()
                Button("Save") { Task { await save() } }
                    .buttonStyle(.borderedProminent)
                    .keyboardShortcut(.defaultAction)
                    .disabled(id.isEmpty || store.isSaving)
            }
        }
        .padding(Theme.Spacing.xl)
        .frame(width: Theme.Size.sheetWidth)
        .presentationSizing(.form)
    }

    private func save() async {
        let path = specPath
        let id = id
        let (name, when) = (name, when)
        let data = await Task.detached { FileManager.default.contents(atPath: path) }.value
        guard
            let data, data.count <= DeliverableStore.sizeCap,
            let json = TemplateDocument.json(from: data, id: id, name: name, when: when)
        else {
            unreadable = true
            return
        }
        unreadable = false
        if await store.save(json: json) { onDone() }
    }
}
