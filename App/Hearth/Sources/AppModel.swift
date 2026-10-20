// AppModel is the app's wiring: it follows the helper supervisor and, for each
// helper that comes up, builds a Session, the stores over that helper's
// client, all fed by the app's one EventRelay. A new helper numbers its events
// afresh and answers on a new port, so the relay is reset over the new client
// and the stores are made again rather than re-pointed; the selection is
// carried across. While no helper runs there is no session and the window says
// so. A helper that comes back after one the window had a session over raises
// the helper notice, which stays until the owner dismisses it.
// See docs/design.md#window-anatomy, docs/model.md#9-failure-handling

import HearthFeatures
import HearthKit
import HearthNative
import Observation

/// The app's current session and the one flag the menu and the window share.
@MainActor
@Observable
final class AppModel {
    let supervisor: HelperSupervisor
    /// The stores over the running helper; nil while it starts or restarts.
    private(set) var session: Session?
    /// Whether the hire sheet is open; the toolbar and the Team menu set it.
    var isHiring = false
    /// The bot whose Access sheet is open; the toolbar, the Team menu and a
    /// bot's context menu set it.
    var accessBotID: String?
    /// Whether the new-project sheet is open; the toolbar and the File menu
    /// set it.
    var isCreatingProject = false
    /// The project whose edit sheet is open; a project's context menu sets it.
    var editingProjectID: String?
    /// Whether the inspector is open beside the conversation; the toolbar and
    /// the View menu set it.
    var showsInspector = false
    /// Whether the window tells the owner the helper restarted under it.
    private(set) var helperRestarted = false

    @ObservationIgnored let credentials: CredentialSync
    @ObservationIgnored private var relay: EventRelay?
    @ObservationIgnored private var lostSession = false
    @ObservationIgnored private var builtGeneration: Int?
    @ObservationIgnored private var selection: SidebarSelection?

    init(supervisor: HelperSupervisor, keychain: any KeychainStore = Keychain()) {
        self.supervisor = supervisor
        credentials = CredentialSync(keychain: keychain) { [weak supervisor] in
            supervisor?.report($0)
        }
    }

    /// Dismisses the helper notice.
    func dismissHelperNotice() {
        helperRestarted = false
    }

    /// Builds the session for the supervisor's client, or drops it when the
    /// helper has gone.
    func helperChanged() {
        if let current = session?.sidebar.selected { selection = current }
        isHiring = false
        accessBotID = nil
        isCreatingProject = false
        editingProjectID = nil
        guard let client = supervisor.client else {
            lostSession = lostSession || session != nil
            session = nil
            relay?.stop()
            credentials.helperGone()
            return
        }
        if (lostSession || session != nil) && supervisor.generation != builtGeneration {
            helperRestarted = true
        }
        lostSession = false
        if supervisor.generation != builtGeneration {
            credentials.helperStarted(client: client)
        }
        builtGeneration = supervisor.generation
        let relay = self.relay ?? EventRelay(client: client)
        session = Session(client: client, relay: relay, selection: selection)
        if self.relay == nil {
            self.relay = relay
            relay.register { [credentials] event in await credentials.handle(event) }
            relay.start()
        } else {
            relay.reset(client: client)
        }
    }
}

/// The stores over one helper.
@MainActor
@Observable
final class Session {
    /// Where the window is in its life: asking the core, failed to, onboarding
    /// or showing the team.
    enum Phase: Equatable {
        case loading
        case failed(String)
        case onboarding
        case team
    }

    let client: any HearthClient
    let accounts: AccountsStore
    let onboarding: OnboardingStore
    let bots: BotListStore
    /// The org's projects: one store for the session, which the sidebar,
    /// the project sheet and the project conversations read.
    let projects: ProjectsStore
    /// What the sidebar has selected, over the bots and the projects.
    let sidebar: SidebarStore
    /// What waits on the owner: one store for the session, which the sidebar
    /// row, the Inbox screen, the threads and the menu bar read.
    let inbox: InboxStore
    /// What the bots cost: one store for the session, which the inspector,
    /// the Usage pane, the budget cards and the paused marks read.
    let usage: UsageStore
    /// Which model each tier asks for: one store for the session, which the
    /// Models pane and every Access sheet's Advanced choice read.
    let models: ModelsStore

    private var fetchedOrg: Hearth_V1_Org?
    private var failure: String?
    private var checked = false
    @ObservationIgnored let relay: EventRelay
    @ObservationIgnored private var conversations: [String: ConversationStore] = [:]
    @ObservationIgnored private var noteStores: [String: NotesStore] = [:]
    @ObservationIgnored private var handbookStores: [String: HandbookStore] = [:]

    init(client: any HearthClient, relay: EventRelay, selection: SidebarSelection?) {
        self.client = client
        self.relay = relay
        accounts = AccountsStore(client: client, relay: relay)
        onboarding = OnboardingStore(client: client, accounts: accounts)
        usage = UsageStore(client: client, relay: relay)
        models = ModelsStore(client: client, relay: relay)
        bots = BotListStore(client: client, relay: relay)
        projects = ProjectsStore(client: client, relay: relay)
        sidebar = SidebarStore(bots: bots, projects: projects, selected: selection)
        inbox = InboxStore(client: client, relay: relay, usage: usage)
        bots.usage = usage
    }

    /// The org, from the core or from onboarding just now.
    var org: Hearth_V1_Org? {
        fetchedOrg ?? onboarding.org
    }

    var phase: Phase {
        if org != nil { return .team }
        if let failure { return .failed(failure) }
        return checked ? .onboarding : .loading
    }

    /// Asks the core whether an org exists yet.
    func loadOrg() async {
        do {
            fetchedOrg = try await client.getOrg()
            failure = nil
            checked = true
        } catch {
            failure = error.localizedDescription
        }
    }

    /// The bot that answers project conversations, the Chief of Staff, if
    /// the org names one and it is on the team.
    var coordinator: Hearth_V1_Bot? {
        guard let id = org?.coordinatorBotID, !id.isEmpty else { return nil }
        return bots.bots.first { $0.id == id }
    }

    /// The store for the conversation with `conversationID`, a bot's DM or a
    /// project's, kept so its transcript and run survive selecting something
    /// else and coming back. `botID` is whose budget pauses it.
    func conversation(for conversationID: String, botID: String?) -> ConversationStore {
        if let store = conversations[conversationID] { return store }
        let store = ConversationStore(
            client: client, relay: relay, conversationID: conversationID,
            inbox: inbox, usage: usage, botID: botID)
        conversations[conversationID] = store
        return store
    }

    /// The notes store for `bot`, kept so the inspector does not refetch each
    /// time the owner comes back to someone.
    func notes(for bot: Hearth_V1_Bot) -> NotesStore {
        if let store = noteStores[bot.id] { return store }
        let store = NotesStore(client: client, relay: relay, botID: bot.id)
        noteStores[bot.id] = store
        return store
    }

    /// The handbook store for `bot`, kept for the same reason as its notes.
    func handbook(for bot: Hearth_V1_Bot) -> HandbookStore {
        if let store = handbookStores[bot.id] { return store }
        let store = HandbookStore(
            client: client, relay: relay, botID: bot.id, botName: bot.name)
        handbookStores[bot.id] = store
        return store
    }
}
