// ModelsPane is the Settings pane that says what Fast, Balanced and Best mean:
// a row per tier, with its name, what it means and a menu of the models it may
// use, grouped by provider, and one caption under them. The rows sit in a
// plain stack separated by hairlines, as the Access rows do, since each holds
// a control and none is a field of a form. The pane is a function of store
// state; a pick goes to the store, which sends it.
// See docs/design.md#forms-sheets-and-settings

import HearthDesign
import HearthKit
import HearthUI
import SwiftUI

/// The Settings pane for the tier to model mapping.
public struct ModelsPane: View {
    let store: ModelsStore

    public init(store: ModelsStore) {
        self.store = store
    }

    public var body: some View {
        VStack(alignment: .leading, spacing: Theme.Spacing.md) {
            VStack(spacing: 0) {
                ForEach(ModelsStore.order, id: \.self) { tier in
                    if tier != ModelsStore.order.first { Divider() }
                    if let current = store.selection(for: tier) {
                        TierRow(
                            tier: tier, options: store.options,
                            selection: Binding(
                                get: { current },
                                set: { pick in Task { await store.set(tier: tier, option: pick) } }
                            ))
                    }
                }
            }
            Text(
                "New bots use Balanced. Pick a model for one bot under Advanced in its Access sheet."
            )
            .font(Theme.caption)
            .foregroundStyle(Theme.color(.textSecondary))
            if let error = store.error {
                Label(error, systemImage: "exclamationmark.triangle")
                    .font(Theme.callout)
                    .foregroundStyle(Theme.color(.toneNegative))
            }
        }
        .task { await store.load() }
    }
}
