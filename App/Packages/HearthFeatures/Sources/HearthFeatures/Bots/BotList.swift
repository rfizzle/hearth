// BotList is the bots' part of the sidebar: the org-level bots, the Chief of
// Staff first, under an "Office" header, then the rest under "Your team", one
// row each with an avatar, the name, the role and, while a bot is working,
// paused or waiting for the owner, a status dot. It is sections, not a list:
// SidebarList holds them in its List with the selection. Rows are List rows,
// so selection, keyboard and VoiceOver are the system's, and a row reads as one
// element, saying "waiting for you" as its value while the bot waits. The role
// uses the system's .secondary style rather than the text token so sidebar
// vibrancy applies over the glass pane. With nobody on the team its section
// shows only the header; the invitation to hire is TeamEmptyView, which the
// window shows in the detail column: one line, an ExampleCard for each role
// template the core serves (a press opens the hire setup with it picked) and
// Hire Someone… under them. A bot's context menu offers Access… when
// the window gives it.
// See docs/design.md#window-anatomy and docs/model.md#owner-interaction

import HearthDesign
import HearthKit
import HearthUI
import SwiftUI

/// The bots' sections of the sidebar.
public struct BotList: View {
    let store: BotListStore
    let onAccess: ((Hearth_V1_Bot) -> Void)?

    /// The Office and the team over `store`. A bot's context menu offers
    /// Access… when `onAccess` is given.
    public init(store: BotListStore, onAccess: ((Hearth_V1_Bot) -> Void)? = nil) {
        self.store = store
        self.onAccess = onAccess
    }

    public var body: some View {
        if !store.office.isEmpty {
            Section("Office") {
                ForEach(store.office, id: \.id) { row($0) }
            }
        }
        Section("Your team") {
            ForEach(store.team, id: \.id) { row($0) }
        }
    }

    private func row(_ bot: Hearth_V1_Bot) -> some View {
        BotRow(bot: bot, activity: store.activity(of: bot))
            .tag(SidebarSelection.bot(bot.id))
            .contextMenu {
                if let onAccess {
                    Button("Access…", systemImage: "lock.open") { onAccess(bot) }
                }
            }
    }
}

/// The detail column's empty state when nobody is hired yet.
public struct TeamEmptyView: View {
    let templates: [Hearth_V1_RoleTemplate]
    let onHire: () -> Void
    let onPick: (Hearth_V1_RoleTemplate) -> Void

    /// The invitation, with a card for each of `templates` (at most four);
    /// `onPick` starts the hire setup with that role, `onHire` with none.
    public init(
        templates: [Hearth_V1_RoleTemplate] = [], onHire: @escaping () -> Void,
        onPick: @escaping (Hearth_V1_RoleTemplate) -> Void = { _ in }
    ) {
        self.templates = templates
        self.onHire = onHire
        self.onPick = onPick
    }

    public var body: some View {
        VStack(spacing: Theme.Spacing.lg) {
            Image(systemName: "person.2")
                .font(.system(size: 40))
                .foregroundStyle(Theme.color(.textSecondary))
                .accessibilityHidden(true)
            Text("Nobody here yet")
                .font(Theme.title)
            Text(line)
                .font(Theme.body)
                .foregroundStyle(Theme.color(.textSecondary))
            if !templates.isEmpty {
                LazyVGrid(
                    columns: [GridItem(.adaptive(minimum: 220), spacing: Theme.Spacing.md)],
                    spacing: Theme.Spacing.md
                ) {
                    ForEach(templates.prefix(4), id: \.id) { template in
                        ExampleCard(
                            symbol: Self.symbol(for: template), title: template.role,
                            line: template.summary
                        ) { onPick(template) }
                    }
                }
                .frame(maxWidth: 520)
            }
            Button("Hire Someone…", action: onHire)
                .buttonStyle(.borderedProminent)
        }
        .padding(Theme.Spacing.xl)
        .frame(maxWidth: .infinity, maxHeight: .infinity)
    }

    private var line: String {
        templates.isEmpty
            ? "Hire someone to get started." : "Start with a role, or describe your own."
    }

    /// A symbol for a role: by what the role is called, else a person.
    static func symbol(for template: Hearth_V1_RoleTemplate) -> String {
        let role = template.role.lowercased()
        let symbols = [
            ("research", "magnifyingglass"), ("writ", "square.and.pencil"),
            ("schedul", "calendar"), ("edit", "pencil.line"), ("account", "chart.pie"),
            ("book", "book"), ("plan", "map"), ("assist", "tray.full"),
            ("market", "megaphone"), ("financ", "chart.bar.doc.horizontal"),
        ]
        return symbols.first { role.contains($0.0) }?.1 ?? "person.crop.circle"
    }
}

private struct BotRow: View {
    let bot: Hearth_V1_Bot
    let activity: BotListStore.Activity?

    var body: some View {
        HStack(spacing: Theme.Spacing.sm) {
            Avatar(bot: bot)
            VStack(alignment: .leading, spacing: 0) {
                Text(bot.name).font(Theme.body)
                Text(bot.role).font(Theme.caption).foregroundStyle(.secondary)
            }
            Spacer(minLength: Theme.Spacing.sm)
            switch activity {
            case .working:
                StatusDot(.positive, label: "Working", pulses: true)
            case .waiting:
                StatusDot(.attention, label: "Waiting")
            case .paused:
                Image(systemName: "pause.circle")
                    .foregroundStyle(Theme.color(Tone.attention))
                    .accessibilityLabel("Paused")
            case nil:
                EmptyView()
            }
        }
        .accessibilityElement(children: .combine)
        .accessibilityValue(activity?.spoken ?? "")
    }
}
