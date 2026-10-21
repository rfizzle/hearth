// The Motion page plays each state change the app animates: one card per
// change with a button that makes it happen in place. The page follows the
// system's Reduce Motion setting; SwiftUI offers no way to override it for a
// subtree, so the setting is flipped in System Settings, not on the page. A
// still cannot show motion,
// so --play walks the cards, scrolling to each and pressing its button twice,
// 0.8 s apart; a screen recording of that is how the motion is reviewed.
// Each card drives the real unit, not a copy: the same views and stores the
// app uses, fed by the same fixtures.
// See docs/design.md#motion

import HearthDesign
import HearthFeatures
import HearthKit
import HearthUI
import SwiftUI

/// A flag a card's button flips.
@MainActor @Observable
final class MotionFlag {
    var isOn = false
}

/// A store over an event relay the card feeds by hand, so a bot or a message
/// can arrive when its button is pressed. Starting over builds a fresh store.
@MainActor @Observable
final class MotionFeed<Store> {
    private(set) var store: Store
    /// Changes with every fresh store, so the view starts over with it.
    private(set) var generation = 0

    @ObservationIgnored private let make: (EventRelay) -> Store
    @ObservationIgnored private var continuation:
        AsyncThrowingStream<Hearth_V1_SubscribeResponse, Error>.Continuation
    @ObservationIgnored private var sequence: UInt64 = 0

    init(make: @escaping (EventRelay) -> Store) {
        self.make = make
        let (built, continuation) = Self.open(make)
        store = built
        self.continuation = continuation
    }

    /// Delivers `event` to the store as the core would.
    func deliver(_ event: Hearth_V1_SubscribeResponse.OneOf_Event) {
        sequence += 1
        continuation.yield(
            .with {
                $0.sequence = sequence; $0.event = event
            })
    }

    /// Replaces the store with a fresh one.
    func restart() {
        continuation.finish()
        sequence = 0
        let (built, continuation) = Self.open(make)
        store = built
        self.continuation = continuation
        generation += 1
    }

    private static func open(
        _ make: (EventRelay) -> Store
    ) -> (Store, AsyncThrowingStream<Hearth_V1_SubscribeResponse, Error>.Continuation) {
        let (stream, continuation) = AsyncThrowingStream<Hearth_V1_SubscribeResponse, Error>
            .makeStream()
        let relay = EventRelay(subscribe: { _ in stream })
        relay.start()
        return (make(relay), continuation)
    }
}

/// One change on the page: its card, and what its button does.
struct MotionStep {
    let state: GalleryState
    let play: @MainActor () -> Void
}

/// A card: the unit, then the button that changes it and the transition used.
private struct MotionCard<Content: View>: View {
    let button: String
    let note: String
    let action: () -> Void
    @ViewBuilder let content: Content

    var body: some View {
        VStack(alignment: .leading, spacing: Theme.Spacing.md) {
            content
            HStack(alignment: .firstTextBaseline, spacing: Theme.Spacing.md) {
                Button(button, action: action)
                Text(note)
                    .font(Theme.caption)
                    .foregroundStyle(Theme.color(.textSecondary))
            }
        }
    }
}

extension Catalog {
    /// The page's name, for the launch argument and the play sequence.
    static let motionPage = "Motion"

    static let motion = GalleryPage(
        package: "HearthDesign", name: motionPage,
        states: motionSteps.map(\.state))

    /// Presses each card's button, on and back off, 0.8 s apart, scrolling to
    /// each card first, after a short lead-in.
    static func playMotion(scrollTo: @MainActor (String) -> Void) async {
        let beat = Duration.milliseconds(800)
        // Time for a recorder to start on the window.
        try? await Task.sleep(for: .seconds(5))
        for step in motionSteps {
            scrollTo(step.state.id)
            try? await Task.sleep(for: beat)
            step.play()
            try? await Task.sleep(for: beat)
            step.play()
            try? await Task.sleep(for: beat)
        }
    }

    static func step<Content: View>(
        _ name: String, button: String, note: String,
        play: @escaping @MainActor () -> Void, @ViewBuilder content: () -> Content
    ) -> MotionStep {
        MotionStep(
            state: GalleryState(
                name: name, width: 560,
                view: AnyView(
                    MotionCard(button: button, note: note, action: play, content: content)),
                pair: false),
            play: play)
    }

    static let motionSteps: [MotionStep] = [
        disclosure, keyField, cardState, onboardingStep, setupStepMotion, concernMotion,
        composerSwap, statusDot, sidebarInsert, transcriptSend, emptyState,
    ]

    private static var disclosure: MotionStep {
        let flag = MotionFlag()
        struct Demo: View {
            @Bindable var flag: MotionFlag
            var body: some View {
                VStack(alignment: .leading) {
                    DisclosureRow(isExpanded: $flag.isOn) {
                        Text("Searched the web")
                    } content: {
                        Text("3 results for “quarterly filing dates”")
                    }
                    Text("A row below, which slides rather than jumps.")
                        .font(Theme.callout)
                }
            }
        }
        return step(
            "Disclosure opens and closes", button: "Open or close",
            note: "The system's height change under the move spring; the body fades in.",
            play: { flag.isOn.toggle() }, content: { Demo(flag: flag) })
    }

    private static var keyField: MotionStep {
        let flag = MotionFlag()
        struct Demo: View {
            @Bindable var flag: MotionFlag
            var body: some View {
                ProviderCard(
                    account: Fixtures.accounts[2], isEditing: $flag.isOn,
                    onSignIn: {}, onSetKey: { _ in }, onClear: {})
            }
        }
        return step(
            "Key field reveals in a card", button: "Add Key or Cancel",
            note: "The field slides down from the top under the move spring.",
            play: { flag.isOn.toggle() }, content: { Demo(flag: flag) })
    }

    private static var cardState: MotionStep {
        let flag = MotionFlag()
        struct Demo: View {
            let flag: MotionFlag
            var body: some View {
                ProviderCard(
                    account: flag.isOn ? signedIn : Fixtures.accounts[2],
                    isEditing: .constant(false),
                    onSignIn: {}, onSetKey: { _ in }, onClear: {})
            }
            private var signedIn: Hearth_V1_ProviderAccount {
                var account = Fixtures.accounts[2]
                account.state = .signedIn
                account.label = "7f3a"
                return account
            }
        }
        return step(
            "Card state changes", button: "Add or clear the key",
            note: "The pill and Add Key or Clear blur-replace in place under the quick fade.",
            play: { flag.isOn.toggle() }, content: { Demo(flag: flag) })
    }

    private static var onboardingStep: MotionStep {
        let client = PreviewClient()
        let accounts = AccountsStore(client: client, relay: EventRelay(client: client))
        let store = OnboardingStore(client: client, accounts: accounts)
        store.path = Fixtures.workspacePath
        Task {
            await accounts.load()
            store.next()
        }
        return step(
            "Onboarding step changes", button: "Continue or Back",
            note: "The step cross-fades while the column's height moves with it.",
            play: { store.step == .signIn ? store.next() : store.back() },
            content: { OnboardingScreen(store: store).frame(height: 600) })
    }

    private static var composerSwap: MotionStep {
        let flag = MotionFlag()
        struct Demo: View {
            let flag: MotionFlag
            var body: some View {
                Composer(
                    label: "Message Dana", isWorking: flag.isOn,
                    draft: "Add a section on the free plans.",
                    onSend: { _ in true }, onStop: {})
            }
        }
        return step(
            "Send becomes Stop", button: "Send or Stop",
            note: "One round button whose symbol replaces itself under the quick fade.",
            play: { flag.isOn.toggle() }, content: { Demo(flag: flag) })
    }

    private static var statusDot: MotionStep {
        let flag = MotionFlag()
        struct Demo: View {
            let flag: MotionFlag
            var body: some View {
                HStack {
                    StatusDot(.positive, label: "Working", pulses: flag.isOn)
                    Text(flag.isOn ? "Working" : "Idle").font(Theme.callout)
                }
            }
        }
        return step(
            "Status dot pulses", button: "Start or stop work",
            note: "The system's repeating pulse; static under Reduce Motion, and the label stays.",
            play: { flag.isOn.toggle() }, content: { Demo(flag: flag) })
    }

    private static var sidebarInsert: MotionStep {
        let feed = MotionFeed { relay in
            let client = PreviewClient()
            return SidebarStore(
                bots: BotListStore(client: client, relay: relay),
                projects: ProjectsStore(client: client, relay: relay))
        }
        struct Demo: View {
            let feed: MotionFeed<SidebarStore>
            var body: some View {
                SidebarList(store: feed.store)
                    .id(feed.generation)
                    .frame(width: Theme.Size.sidebarIdeal, height: 200)
            }
        }
        return step(
            "Bot appears in the sidebar", button: "Hire someone",
            note: "The row slides into the list under the move spring; pressing again starts over.",
            play: {
                if feed.store.bots.bots.count > 1 {
                    feed.restart()
                } else {
                    feed.deliver(
                        .botChanged(
                            .with {
                                $0.bot = .with {
                                    $0.id = "bot-priya"
                                    $0.name = "Priya"
                                    $0.role = "Bookkeeper"
                                    $0.status = .idle
                                }
                            }))
                }
            }, content: { Demo(feed: feed) })
    }

    private static var transcriptSend: MotionStep {
        let feed = MotionFeed { conversation("conversation-dana", relay: $0) }
        return step(
            "Message appears in the transcript", button: "Send a message",
            note: "The owner's message rises in under the quick fade; bot rows appear in place.",
            play: {
                if feed.store.messages.contains(where: { $0.id.hasPrefix("message-owner") }) {
                    feed.restart()
                } else {
                    feed.deliver(ownerMessage(in: "conversation-dana", seq: 11))
                }
            }, content: { ConversationDemo(feed: feed) })
    }

    private static var emptyState: MotionStep {
        let feed = MotionFeed { conversation("empty", relay: $0) }
        return step(
            "Empty state gives way", button: "Send the first message",
            note: "The empty state fades out and the transcript is simply there.",
            play: {
                if feed.store.rows.isEmpty {
                    feed.deliver(ownerMessage(in: "empty", seq: 1))
                } else {
                    feed.restart()
                }
            }, content: { ConversationDemo(feed: feed) })
    }

    private static func conversation(_ id: String, relay: EventRelay) -> ConversationStore {
        ConversationStore(client: PreviewClient(), relay: relay, conversationID: id)
    }

    private static func ownerMessage(
        in conversation: String, seq: Int64
    ) -> Hearth_V1_SubscribeResponse.OneOf_Event {
        .messageUpserted(
            .with {
                $0.message = .with {
                    $0.id = "message-owner-\(seq)"
                    $0.conversationID = conversation
                    $0.seq = seq
                    $0.kind = .text
                    $0.content = "Thanks. Please add a short summary at the top."
                }
            })
    }
}

private struct ConversationDemo: View {
    let feed: MotionFeed<ConversationStore>

    var body: some View {
        ConversationScreen(store: feed.store, bot: Fixtures.bot)
            .id(feed.generation)
            .frame(height: 360)
    }
}
