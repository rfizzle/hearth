// HireSheet shows the store's form: a heading and a lead, a grid of template
// cards with a "Describe the job" card among them, then a grouped form for
// name, role, job description, quality and avatar. The cards are the choice
// and the form is plain text input, so only the second is a Form. The whole
// body scrolls, and Cancel and the one prominent Create float on a scroll
// edge at the bottom: Escape cancels, Return creates once a name and a job
// description are typed. Under the form, DraftPanel offers "Improve this" and
// shows the core's suggestion beside the owner's text. The sheet is a function
// of store state; it reports the new bot to its caller and has no local state
// beyond the editor's height.
// See docs/design.md#forms-sheets-and-settings

import HearthDesign
import HearthKit
import HearthUI
import SwiftUI

/// The sheet that hires one bot.
public struct HireSheet: View {
    let store: HireStore
    let onHired: (Hearth_V1_Bot) -> Void
    let onCancel: () -> Void

    /// Four lines of body text, so the job description has room to be written.
    @ScaledMetric(relativeTo: .body) private var editorHeight: CGFloat = 80

    public init(
        store: HireStore, onHired: @escaping (Hearth_V1_Bot) -> Void,
        onCancel: @escaping () -> Void
    ) {
        self.store = store
        self.onHired = onHired
        self.onCancel = onCancel
    }

    public var body: some View {
        @Bindable var store = store
        ScrollView {
            VStack(alignment: .leading, spacing: Theme.Spacing.lg) {
                VStack(alignment: .leading, spacing: Theme.Spacing.xs) {
                    Text("Hire someone").font(Theme.title)
                    Text("Pick a role to start from, or describe the job yourself.")
                        .font(Theme.body)
                        .foregroundStyle(Theme.color(.textSecondary))
                }
                .padding(.horizontal, Theme.Spacing.xl)
                cards
                    .padding(.horizontal, Theme.Spacing.xl)
                form(fields: $store)
                DraftPanel(store: store)
                    .padding(.horizontal, Theme.Spacing.xl)
            }
            .padding(.top, Theme.Spacing.xl)
        }
        .scrollEdgeEffectStyle(.soft, for: .bottom)
        .safeAreaBar(edge: .bottom) { buttons }
        .frame(width: Theme.Size.sheetWidth)
        .presentationSizing(.form)
        .task { await store.load() }
    }

    private var cards: some View {
        LazyVGrid(
            columns: [GridItem(.adaptive(minimum: 150), spacing: Theme.Spacing.md)],
            spacing: Theme.Spacing.md
        ) {
            ForEach(store.templates, id: \.id) { template in
                TemplateCard(
                    template: template, isSelected: store.start == .template(template.id),
                    onPick: { store.pick(template) })
            }
            TemplateCard(
                role: "Describe the job",
                summary: "Start from a blank page and say what you need.",
                isSelected: store.start == .describe, isBlank: true,
                onPick: { store.describe() })
        }
    }

    private func form(fields: Bindable<HireStore>) -> some View {
        Form {
            Section {
                TextField("Name", text: fields.name)
                TextField("Role", text: fields.role)
                VStack(alignment: .leading, spacing: Theme.Spacing.xs) {
                    Text("Job description")
                    CardSurface {
                        TextEditor(text: fields.jobDescription)
                            .font(Theme.body)
                            .scrollContentBackground(.hidden)
                            .frame(minHeight: editorHeight)
                            .accessibilityLabel("Job description")
                    }
                }
                TierPicker(tier: fields.tier)
                TextField(
                    "Avatar", text: fields.avatar,
                    prompt: Text("An emoji or a symbol name; empty for initials"))
                if let error = store.error {
                    Text(error).foregroundStyle(Theme.color(.toneNegative))
                }
            }
        }
        .formStyle(.grouped)
        .scrollDisabled(true)
    }

    private var buttons: some View {
        HStack {
            Button("Cancel", action: onCancel)
                .keyboardShortcut(.cancelAction)
            Spacer()
            Button("Create") { Task { await create() } }
                .buttonStyle(.borderedProminent)
                .keyboardShortcut(.defaultAction)
                .disabled(!store.canCreate)
        }
        .padding(.horizontal, Theme.Spacing.xl)
        .padding(.vertical, Theme.Spacing.md)
    }

    private func create() async {
        await store.create()
        if let bot = store.createdBot { onHired(bot) }
    }
}
