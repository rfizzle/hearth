// TemplatesPane is Settings › Deliverables: the templates Hearth ships as
// cards, each with its name, the sentence on when to use it and a small
// rendering of its sample, then the owner's own below, with Delete on each
// and, for one that fails to load, the core's problem in a line. "Show in
// Finder" opens the folder the owner's templates live in. There is no event
// for a template, so the list is read again each time the window becomes key.
// A shipped template's sample is the copy Hearth carries; the core sends the
// text only for the owner's.
// See docs/design.md#forms-sheets-and-settings, docs/decisions.md (D43)

import HearthDesign
import HearthKit
import HearthUI
import SwiftUI

/// The Settings pane for deliverable templates.
public struct TemplatesPane: View {
    let store: TemplatesStore

    @Environment(\.revealInFinder) private var revealInFinder
    @Environment(\.controlActiveState) private var activeState

    public init(store: TemplatesStore) {
        self.store = store
    }

    public var body: some View {
        VStack(alignment: .leading, spacing: Theme.Spacing.lg) {
            Text("Shipped with Hearth").font(Theme.headline)
            ForEach(store.shipped, id: \.id) { TemplateCard(template: $0, onDelete: nil) }
            HStack {
                Text("Yours").font(Theme.headline)
                Spacer()
                if let folder = store.folderPath {
                    Button("Show in Finder", systemImage: "folder") { revealInFinder(folder) }
                        .buttonStyle(.link)
                }
            }
            if store.owned.isEmpty {
                Text("Open a deliverable and choose Save as Template in its More menu.")
                    .font(Theme.callout)
                    .foregroundStyle(Theme.color(.textSecondary))
            }
            ForEach(store.owned, id: \.id) { template in
                TemplateCard(template: template) { Task { await store.delete(id: template.id) } }
            }
            if let error = store.error {
                Label(error, systemImage: "exclamationmark.triangle")
                    .font(Theme.callout)
                    .foregroundStyle(Theme.color(.toneNegative))
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .task { await store.load() }
        .onChange(of: activeState) { _, state in
            if state == .key { Task { await store.load() } }
        }
    }
}

/// One template: a thumbnail of its sample, its name and `when`, and, for the
/// owner's, Delete or the problem that kept it from loading.
struct TemplateCard: View {
    let template: Hearth_V1_DeliverableTemplate
    let onDelete: (() -> Void)?

    /// The width the sample is laid out at before it is shrunk to a thumbnail.
    private static let layoutWidth: CGFloat = 640
    private static let scale: CGFloat = 0.3

    var body: some View {
        CardSurface {
            HStack(alignment: .top, spacing: Theme.Spacing.md) {
                thumbnail
                VStack(alignment: .leading, spacing: Theme.Spacing.xs) {
                    Text(template.name.isEmpty ? template.id : template.name)
                        .font(Theme.headline)
                        .foregroundStyle(Theme.color(.textPrimary))
                    if !template.when.isEmpty {
                        Text(template.when)
                            .font(Theme.callout)
                            .foregroundStyle(Theme.color(.textSecondary))
                    }
                    if !template.problem.isEmpty {
                        Label(template.problem, systemImage: "exclamationmark.triangle")
                            .font(Theme.callout)
                            .foregroundStyle(Theme.color(.toneNegative))
                    }
                    if let onDelete {
                        Button("Delete", role: .destructive, action: onDelete)
                            .padding(.top, Theme.Spacing.xs)
                    }
                }
                Spacer(minLength: 0)
            }
        }
        .accessibilityElement(children: .contain)
        .accessibilityLabel(template.name.isEmpty ? template.id : template.name)
    }

    @ViewBuilder private var thumbnail: some View {
        if let spec = sample {
            DeliverableView(spec: spec)
                .frame(width: Self.layoutWidth, alignment: .topLeading)
                .scaleEffect(Self.scale, anchor: .topLeading)
                .frame(
                    width: Self.layoutWidth * Self.scale, height: 150, alignment: .topLeading
                )
                .clipShape(RoundedRectangle(cornerRadius: Theme.Radius.sm))
                .allowsHitTesting(false)
                .accessibilityHidden(true)
        } else {
            Image(systemName: "doc.questionmark")
                .font(.title)
                .foregroundStyle(Theme.color(.textSecondary))
                .frame(width: Self.layoutWidth * Self.scale, height: 150)
                .accessibilityHidden(true)
        }
    }

    /// The sample to draw: Hearth's copy of a shipped template, the owner's
    /// own text otherwise; nil when there is none that reads.
    private var sample: Deliverable? {
        if template.shipped {
            return Fixtures.shippedDeliverableIDs.contains(template.id)
                ? Fixtures.deliverable(template.id) : nil
        }
        return try? Deliverable(json: Data(template.json.utf8))
    }
}
