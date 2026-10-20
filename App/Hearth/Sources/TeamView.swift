// TeamView is the window once the org exists: the bots in the sidebar and the
// selected bot's conversation in the detail column, with Hire Someone… as an
// icon-only toolbar item over the sidebar and the bot's folder as the one
// item beside Access… and the inspector toggle over the conversation. The
// inspector opens beside it with what the bot remembers. The Inbox row opens
// the Inbox in the detail column. Nobody on the team shows the invitation in the
// detail column; nobody selected says to choose. The hire sheet and the Access
// sheet are presented here, and the helper notice sits over the top of the
// detail column until the owner dismisses it. When the window first shows
// the team with nothing selected, the Chief of Staff is.
// See docs/design.md#window-anatomy and docs/model.md#9-failure-handling

import HearthDesign
import HearthFeatures
import HearthKit
import HearthUI
import SwiftUI

struct TeamView: View {
    let session: Session
    @Environment(AppModel.self) private var model
    @Environment(\.revealInFinder) private var revealInFinder
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    var body: some View {
        @Bindable var model = model
        NavigationSplitView {
            BotList(
                store: session.bots, inbox: session.inbox,
                onAccess: { model.accessBotID = $0.id },
                onHire: { model.isHiring = true }
            )
            .navigationSplitViewColumnWidth(
                min: Theme.Size.sidebarMin, ideal: Theme.Size.sidebarIdeal,
                max: Theme.Size.sidebarMax
            )
            .toolbar {
                ToolbarItem(placement: .primaryAction) {
                    Button("Hire Someone…", systemImage: "person.badge.plus") {
                        model.isHiring = true
                    }
                    .labelStyle(.iconOnly)
                    .help("Hire someone")
                }
            }
        } detail: {
            detail
                .safeAreaInset(edge: .top, spacing: 0) {
                    if model.helperRestarted {
                        HelperBanner(onDismiss: model.dismissHelperNotice)
                            .transition(.move(edge: .top).combined(with: .opacity))
                    }
                }
                .animation(
                    Theme.motion(Theme.quick, reduceMotion: reduceMotion),
                    value: model.helperRestarted)
        }
        .navigationSplitViewStyle(.balanced)
        .onChange(of: session.bots.office.first?.id, initial: true) {
            session.bots.selectOfficeIfNothingSelected()
        }
        .task { await session.usage.load() }
        .sheet(isPresented: $model.isHiring) {
            HireSheetHost(session: session)
        }
        .sheet(isPresented: isChangingAccess) {
            if let bot = accessBot { AccessSheetHost(session: session, bot: bot) }
        }
    }

    /// Whether the Access sheet is open for a bot still on the team.
    private var isChangingAccess: Binding<Bool> {
        Binding(
            get: { accessBot != nil },
            set: { if !$0 { model.accessBotID = nil } })
    }

    private var accessBot: Hearth_V1_Bot? {
        session.bots.bots.first { $0.id == model.accessBotID }
    }

    @ViewBuilder
    private var detail: some View {
        @Bindable var model = model
        if session.bots.selected == .inbox {
            InboxScreen(store: session.inbox, bots: session.bots.bots)
        } else if let bot = session.bots.selectedBot {
            ConversationScreen(store: session.conversation(for: bot), bot: bot)
                .id(bot.id)
                .navigationSubtitle(bot.role)
                .inspector(isPresented: $model.showsInspector) {
                    InspectorView(
                        notes: session.notes(for: bot), handbook: session.handbook(for: bot),
                        usage: session.usage, botName: bot.name
                    )
                    .inspectorColumnWidth(
                        min: Theme.Size.inspectorMin, ideal: Theme.Size.inspectorIdeal,
                        max: Theme.Size.inspectorMax)
                }
                .toolbar {
                    ToolbarItem(placement: .primaryAction) {
                        Button("Access…", systemImage: "lock.open") {
                            model.accessBotID = bot.id
                        }
                        .labelStyle(.iconOnly)
                        .help("What \(bot.name) can see and do")
                    }
                    ToolbarItem(placement: .primaryAction) {
                        Button("Show Folder in Finder", systemImage: "folder") {
                            revealInFinder(bot.folderPath)
                        }
                        .labelStyle(.iconOnly)
                        .help("Show \(bot.name)'s folder in Finder")
                    }
                    ToolbarItem(placement: .primaryAction) {
                        Button("Show Inspector", systemImage: "sidebar.trailing") {
                            model.showsInspector.toggle()
                        }
                        .labelStyle(.iconOnly)
                        .help("Show or hide what \(bot.name) remembers")
                    }
                }
        } else if session.bots.showsHireAffordance {
            TeamEmptyView(onHire: { model.isHiring = true })
                .navigationTitle(session.org?.name ?? "Hearth")
        } else {
            ContentUnavailableView(
                "No one selected", systemImage: "bubble.left.and.bubble.right",
                description: Text("Choose someone in the sidebar.")
            )
            .navigationTitle(session.org?.name ?? "Hearth")
        }
    }
}

/// One presentation of the hire sheet, with a form of its own each time.
private struct HireSheetHost: View {
    let session: Session
    @Environment(AppModel.self) private var model
    @State private var store: HireStore

    init(session: Session) {
        self.session = session
        _store = State(initialValue: HireStore(client: session.client))
    }

    var body: some View {
        HireSheet(
            store: store,
            onHired: { bot in
                session.bots.selection = bot.id
                model.isHiring = false
            },
            onCancel: { model.isHiring = false })
    }
}

/// One presentation of the Access sheet, over a grants store made for it.
private struct AccessSheetHost: View {
    let bot: Hearth_V1_Bot
    @Environment(AppModel.self) private var model
    @State private var store: GrantsStore
    @State private var advanced: BotModelStore

    init(session: Session, bot: Hearth_V1_Bot) {
        self.bot = bot
        _store = State(
            initialValue: GrantsStore(
                client: session.client, relay: session.relay, botID: bot.id))
        _advanced = State(
            initialValue: BotModelStore(
                client: session.client, models: session.models, bot: bot))
    }

    var body: some View {
        AccessSheet(
            store: store, advanced: advanced, botName: bot.name, onDone: { model.accessBotID = nil }
        )
    }
}
