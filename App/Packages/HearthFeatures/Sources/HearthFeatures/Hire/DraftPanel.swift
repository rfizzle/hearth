// DraftPanel is the hire sheet's "Improve this" step. At rest it is one
// bordered button and a line saying what it does. With a proposal it sets the
// owner's text and the core's suggestion side by side, as two cards: the
// owner's, read only and exactly as typed, and the suggestion with its name,
// role, job description and working style as fields they can still change,
// and the quality it suggests, which the form's own picker changes after. A
// row of working-style chips adds one sentence to the suggestion's job
// description, in plain view. "Use this" moves the suggestion into the
// form and "Keep mine" drops it; nothing reaches the form any other way, so
// Create sends the text the owner has seen.
// See docs/model.md#people and docs/design.md#forms-sheets-and-settings

import HearthDesign
import HearthKit
import SwiftUI

struct DraftPanel: View {
    @Bindable var store: HireStore

    /// Four lines of body text, so the suggestion has room to be edited.
    @ScaledMetric(relativeTo: .body) private var editorHeight: CGFloat = 96

    var body: some View {
        if let proposal = Binding($store.proposal) {
            suggestion(proposal)
        } else {
            offer
        }
    }

    private var offer: some View {
        HStack(spacing: Theme.Spacing.md) {
            Button {
                Task { await store.improve() }
            } label: {
                Label("Improve this", systemImage: "sparkles")
            }
            .buttonStyle(.bordered)
            .disabled(!store.canDraft)
            if store.isDrafting { ProgressView().controlSize(.small) }
            Text(
                store.isDrafting
                    ? "Writing a suggestion…"
                    : "Get a suggested name, a clearer description and a way of working. You choose what to keep."
            )
            .font(Theme.callout)
            .foregroundStyle(Theme.color(.textSecondary))
        }
    }

    private func suggestion(_ proposal: Binding<HireStore.Proposal>) -> some View {
        VStack(alignment: .leading, spacing: Theme.Spacing.md) {
            HStack(alignment: .top, spacing: Theme.Spacing.md) {
                CardSurface {
                    VStack(alignment: .leading, spacing: Theme.Spacing.sm) {
                        heading("Yours")
                        Text(store.jobDescription)
                            .font(Theme.body)
                            .textSelection(.enabled)
                            .frame(maxWidth: .infinity, alignment: .leading)
                    }
                }
                CardSurface(isSelected: true) {
                    VStack(alignment: .leading, spacing: Theme.Spacing.sm) {
                        HStack {
                            heading("Suggested")
                            Spacer()
                            Pill(Self.quality(proposal.wrappedValue.tier))
                        }
                        TextField("Name", text: proposal.name)
                        TextField("Role", text: proposal.role)
                        TextEditor(text: proposal.jobDescription)
                            .font(Theme.body)
                            .scrollContentBackground(.hidden)
                            .frame(minHeight: editorHeight)
                            .accessibilityLabel("Suggested job description")
                        TextField("Working style", text: proposal.workingStyle, axis: .vertical)
                    }
                    .textFieldStyle(.roundedBorder)
                }
            }
            chips(chosen: proposal.wrappedValue.style)
            HStack {
                Spacer()
                Button("Keep mine") { store.keepMine() }
                    .buttonStyle(.bordered)
                Button("Use this") { store.useProposal() }
                    .buttonStyle(.bordered)
            }
        }
    }

    private func chips(chosen: HireStore.Style?) -> some View {
        VStack(alignment: .leading, spacing: Theme.Spacing.xs) {
            Text("Add a way of working to the suggestion")
                .font(Theme.callout)
                .foregroundStyle(Theme.color(.textSecondary))
            HStack(spacing: Theme.Spacing.sm) {
                ForEach(HireStore.Style.allCases, id: \.self) { style in
                    Button {
                        store.choose(style)
                    } label: {
                        Pill(style.title, tone: .neutral)
                            .overlay {
                                if chosen == style {
                                    Capsule().strokeBorder(.tint, lineWidth: 2)
                                }
                            }
                    }
                    .buttonStyle(.plain)
                    .help(style.sentence)
                    .accessibilityAddTraits(chosen == style ? .isSelected : [])
                }
            }
        }
    }

    private static func quality(_ tier: Hearth_V1_Tier) -> String {
        switch tier {
        case .fast: "Fast"
        case .best: "Best"
        default: "Balanced"
        }
    }

    private func heading(_ text: String) -> some View {
        Text(text)
            .font(Theme.caption.weight(.medium))
            .foregroundStyle(Theme.color(.textSecondary))
    }
}
