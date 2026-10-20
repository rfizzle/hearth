// ProposalCard is a project the Chief of Staff proposed, waiting for one
// click, stacked above the status line in the conversation it was proposed in.
// It is a CardSurface with a folder mark, "Project proposed" in secondary
// caption type, the name, the "done when" sentence and a Brief disclosure row
// with the rest; Create project is the prominent button and Not now the plain
// one. The card decides nothing: Create confirms the proposal in the core,
// Not now sets it aside. It is one VoiceOver container named by the project,
// and both buttons keep their own roles.
// See docs/model.md#work

import HearthDesign
import HearthKit
import SwiftUI

/// A proposed project and the two answers to it.
public struct ProposalCard: View {
    let project: Hearth_V1_Project
    let onCreate: () -> Void
    let onDismiss: () -> Void
    @State private var briefOpen: Bool

    /// A card for the proposed `project`; `briefOpen` starts the Brief row open.
    public init(
        project: Hearth_V1_Project, briefOpen: Bool = false,
        onCreate: @escaping () -> Void, onDismiss: @escaping () -> Void
    ) {
        self.project = project
        self.onCreate = onCreate
        self.onDismiss = onDismiss
        _briefOpen = State(initialValue: briefOpen)
    }

    public var body: some View {
        CardSurface {
            HStack(alignment: .top, spacing: Theme.Spacing.md) {
                Image(systemName: "folder.badge.plus")
                    .font(Theme.title)
                    .foregroundStyle(.tint)
                    .accessibilityHidden(true)
                VStack(alignment: .leading, spacing: Theme.Spacing.xs) {
                    Text("Project proposed")
                        .font(Theme.caption)
                        .foregroundStyle(.secondary)
                    Text(project.name).font(Theme.headline)
                    Text(project.brief.doneWhen).font(Theme.callout)
                    DisclosureRow(
                        isExpanded: $briefOpen,
                        label: { Text("Brief") },
                        content: {
                            VStack(alignment: .leading, spacing: Theme.Spacing.sm) {
                                part("Goal", project.brief.goal)
                                part("Context", project.brief.context)
                                part("What done looks like", project.brief.doneLooksLike)
                            }
                            .frame(maxWidth: .infinity, alignment: .leading)
                            .padding(.bottom, Theme.Spacing.sm)
                        })
                    HStack(spacing: Theme.Spacing.md) {
                        Spacer(minLength: 0)
                        Button("Not now", action: onDismiss)
                            .buttonStyle(.plain)
                            .foregroundStyle(.secondary)
                        Button("Create project", action: onCreate)
                            .buttonStyle(.borderedProminent)
                    }
                }
                .frame(maxWidth: .infinity, alignment: .leading)
            }
        }
        .accessibilityElement(children: .contain)
        .accessibilityLabel("Project proposed: \(project.name)")
    }

    private func part(_ title: String, _ text: String) -> some View {
        VStack(alignment: .leading, spacing: 0) {
            Text(title).font(Theme.caption).foregroundStyle(.secondary)
            Text(text).font(Theme.body)
        }
    }
}

extension ProposalCard {
    /// The states the gallery and previews show.
    public enum PreviewState: String, CaseIterable, Sendable {
        case closed
        case briefOpen = "brief open"
    }

    /// A card showing `state` over the fixtures' proposal, for the gallery and
    /// previews.
    public init(previewState state: PreviewState) {
        self.init(
            project: Fixtures.projects[3], briefOpen: state == .briefOpen, onCreate: {},
            onDismiss: {})
    }
}

#Preview("Closed") { ProposalCard(previewState: .closed).padding() }
#Preview("Brief open") { ProposalCard(previewState: .briefOpen).padding() }
