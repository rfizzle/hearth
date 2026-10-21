// ProducedCard is the small card at a transcript's top edge that lists what the
// conversation produced: the deliverables as chips, then the other files as
// rows, at most five with a "Show all" for the rest (ProducedItems). The header
// folds the card into one "Produced 4 items" pill and back; the host decides
// where it sits and remembers the fold. Folding and listing move with the
// quick token and do nothing under Reduce Motion. It is one VoiceOver
// container named by its count.
// See docs/design.md#the-transcript and docs/design.md#motion

import HearthDesign
import HearthKit
import SwiftUI

/// What a conversation produced, in one card.
public struct ProducedCard: View {
    /// The card's width while open.
    public static let width: CGFloat = 248

    let items: [DeliverableGroup.Item]
    @Binding var isCollapsed: Bool
    @State private var showAll: Bool
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    /// A card for `list`; the host keeps whether it is folded.
    public init(list: ProducedList, isCollapsed: Binding<Bool>) {
        self.init(items: list.items, isCollapsed: isCollapsed)
    }

    init(items: [DeliverableGroup.Item], isCollapsed: Binding<Bool>, showAll: Bool = false) {
        self.items = items
        _isCollapsed = isCollapsed
        _showAll = State(initialValue: showAll)
    }

    public var body: some View {
        VStack(alignment: .leading, spacing: Theme.Spacing.sm) {
            header
            if !isCollapsed {
                ForEach(ProducedItems.visible(items, showAll: showAll), id: \.id) { item in
                    switch item {
                    case .file(let file): FileRow(file: file)
                    case .deliverable(let group): DeliverableChip(group: group)
                    }
                }
                if !showAll, items.count > ProducedItems.limit {
                    Button("Show all \(items.count)") {
                        withAnimation(Theme.motion(Theme.quick, reduceMotion: reduceMotion)) {
                            showAll = true
                        }
                    }
                    .buttonStyle(.plain)
                    .font(Theme.callout)
                    .foregroundStyle(.tint)
                }
            }
        }
        .padding(.horizontal, Theme.Spacing.md)
        .padding(.vertical, Theme.Spacing.sm)
        .frame(width: isCollapsed ? nil : Self.width, alignment: .leading)
        .glassEffect(.regular, in: .rect(cornerRadius: Theme.Radius.lg))
        .accessibilityElement(children: .contain)
    }

    private var header: some View {
        Button {
            withAnimation(Theme.motion(Theme.quick, reduceMotion: reduceMotion)) {
                isCollapsed.toggle()
            }
        } label: {
            HStack(spacing: Theme.Spacing.xs) {
                Image(systemName: "tray.full")
                    .accessibilityHidden(true)
                Text(ProducedItems.pill(count: items.count))
                    .font(Theme.callout.weight(.medium))
                if !isCollapsed { Spacer(minLength: 0) }
                Image(systemName: isCollapsed ? "chevron.down" : "chevron.up")
                    .font(Theme.caption)
                    .accessibilityHidden(true)
            }
            .foregroundStyle(Theme.color(.textPrimary))
            .frame(minHeight: Theme.Size.minTarget)
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
        .accessibilityLabel(ProducedItems.pill(count: items.count))
        .accessibilityHint(isCollapsed ? "Shows the list" : "Hides the list")
    }
}

extension ProducedCard {
    /// The states the gallery and previews show.
    public enum PreviewState: String, CaseIterable, Sendable {
        case open
        case openWithMore = "open with more"
        case collapsed
    }

    /// A card showing `state` over a launch's files, for the gallery and
    /// previews.
    public init(previewState state: PreviewState) {
        let folder = Fixtures.bot.folderPath + "/"
        func file(_ name: String) -> Hearth_V1_FileRef {
            .with {
                $0.name = name
                $0.path = folder + name
            }
        }
        let names = [
            "Launch dashboard.deliverable.json", "Launch dashboard.html", "Launch dashboard.pdf",
            "Customer email draft.md", "API pricing comparison.xlsx", "Launch checklist.md",
            "Press notes.docx", "Support macros.md",
        ]
        let files = names.prefix(state == .openWithMore ? names.count : 5).map(file)
        self.init(
            items: ProducedItems.build(messages: [.with { $0.files = files }]),
            isCollapsed: .constant(state == .collapsed))
    }
}

#Preview("Open") { ProducedCard(previewState: .open).padding() }
#Preview("Open with more") { ProducedCard(previewState: .openWithMore).padding() }
#Preview("Collapsed") { ProducedCard(previewState: .collapsed).padding() }
