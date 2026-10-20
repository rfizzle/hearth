// TeamView is the window once the org exists: the bots in the sidebar and the
// selected bot's conversation in the detail column, with Hire Someone… as an
// icon-only toolbar item over the sidebar and the bot's folder as the one
// item over the conversation. Nobody hired shows the invitation in the detail
// column; nobody selected says to choose. The hire sheet is presented here.
// See docs/design.md#window-anatomy

import HearthDesign
import HearthFeatures
import HearthKit
import HearthUI
import SwiftUI

struct TeamView: View {
    let session: Session
    @Environment(AppModel.self) private var model
    @Environment(\.revealInFinder) private var revealInFinder

    var body: some View {
        @Bindable var model = model
        NavigationSplitView {
            BotList(store: session.bots, onHire: { model.isHiring = true })
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
        }
        .navigationSplitViewStyle(.balanced)
        .sheet(isPresented: $model.isHiring) {
            HireSheetHost(session: session)
        }
    }

    @ViewBuilder
    private var detail: some View {
        if let bot = session.bots.selectedBot {
            ConversationScreen(store: session.conversation(for: bot), bot: bot)
                .id(bot.id)
                .navigationSubtitle(bot.role)
                .toolbar {
                    ToolbarItem(placement: .primaryAction) {
                        Button("Show Folder in Finder", systemImage: "folder") {
                            revealInFinder(bot.folderPath)
                        }
                        .labelStyle(.iconOnly)
                        .help("Show \(bot.name)'s folder in Finder")
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
