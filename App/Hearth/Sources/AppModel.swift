// AppModel is the app's wiring: it follows the helper supervisor and, for each
// helper that comes up, builds a Session, the stores over that helper's
// client, all fed by the app's one EventRelay. A new helper numbers its events
// afresh and answers on a new port, so the relay is reset over the new client
// and the stores are made again rather than re-pointed; the selection is
// carried across. While no helper runs there is no session and the window says
// so.
// See docs/design.md#window-anatomy

import HearthFeatures
import HearthKit
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

    @ObservationIgnored private var relay: EventRelay?
    @ObservationIgnored private var selection: String?

    init(supervisor: HelperSupervisor) {
        self.supervisor = supervisor
    }

    /// Builds the session for the supervisor's client, or drops it when the
    /// helper has gone.
    func helperChanged() {
        if let current = session?.bots.selection { selection = current }
        isHiring = false
        guard let client = supervisor.client else {
            session = nil
            relay?.stop()
            return
        }
        let relay = self.relay ?? EventRelay(client: client)
        session = Session(client: client, relay: relay, selection: selection)
        if self.relay == nil {
            self.relay = relay
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

    private var fetchedOrg: Hearth_V1_Org?
    private var failure: String?
    private var checked = false
    @ObservationIgnored private let relay: EventRelay
    @ObservationIgnored private var conversations: [String: ConversationStore] = [:]

    init(client: any HearthClient, relay: EventRelay, selection: String?) {
        self.client = client
        self.relay = relay
        accounts = AccountsStore(client: client, relay: relay)
        onboarding = OnboardingStore(client: client, accounts: accounts)
        bots = BotListStore(client: client, relay: relay)
        bots.selection = selection
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

    /// The store for `bot`'s conversation, kept so a bot's transcript and run
    /// survive selecting someone else and coming back.
    func conversation(for bot: Hearth_V1_Bot) -> ConversationStore {
        if let store = conversations[bot.dmConversationID] { return store }
        let store = ConversationStore(
            client: client, relay: relay, conversationID: bot.dmConversationID)
        conversations[bot.dmConversationID] = store
        return store
    }
}
