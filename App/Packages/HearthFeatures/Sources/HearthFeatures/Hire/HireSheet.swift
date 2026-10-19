// HireSheet shows the store's form: a grid of template cards with a "Describe
// the job" card among them, then name, role, job description, quality and
// avatar fields. Create stays disabled until a name and a job description are
// typed. The sheet is a function of store state; it reports the new bot to its
// caller and has no local state.

import HearthKit
import HearthUI
import SwiftUI

/// The sheet that hires one bot.
public struct HireSheet: View {
    let store: HireStore
    let onHired: (Hearth_V1_Bot) -> Void
    let onCancel: () -> Void

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
        VStack(alignment: .leading, spacing: 16) {
            Text("Hire someone").font(.title)
            LazyVGrid(columns: [GridItem(.adaptive(minimum: 180), alignment: .top)], spacing: 12) {
                ForEach(store.templates, id: \.id) { template in
                    TemplateCard(
                        template: template, isSelected: store.start == .template(template.id),
                        onPick: { store.pick(template) })
                }
                TemplateCard(
                    role: "Describe the job",
                    summary: "Start from a blank page and say what you need.",
                    isSelected: store.start == .describe, onPick: { store.describe() })
            }
            TextField("Name", text: $store.name)
            TextField("Role", text: $store.role)
            VStack(alignment: .leading, spacing: 4) {
                Text("Job description")
                TextEditor(text: $store.jobDescription)
                    .frame(minHeight: 100)
                    .accessibilityLabel("Job description")
            }
            TierPicker(tier: $store.tier)
            TextField("Avatar (an emoji or a symbol name; empty for initials)", text: $store.avatar)
            if let error = store.error {
                Text(error).foregroundStyle(.primary)
            }
            HStack {
                Button("Cancel", action: onCancel)
                Spacer()
                Button("Create") { Task { await create() } }
                    .buttonStyle(.borderedProminent)
                    .disabled(!store.canCreate)
            }
        }
        .padding()
        .frame(minWidth: 480)
        .task { await store.load() }
    }

    private func create() async {
        await store.create()
        if let bot = store.createdBot { onHired(bot) }
    }
}
