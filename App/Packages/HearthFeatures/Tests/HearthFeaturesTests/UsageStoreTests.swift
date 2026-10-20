// Store tests for usage: PreviewClient's two months, a usage tick moving the
// figures, a budget going out in cents, a pause shown to the sidebar and the
// composer and lifted by Resume (also from the Inbox's card), and a month
// with no price showing tokens.

import Foundation
import HearthKit
import HearthUI
import Testing

@testable import HearthFeatures

@MainActor
private func loadedUsage() async -> UsageStore {
    let client = PreviewClient()
    let store = UsageStore(client: client, relay: EventRelay(client: client))
    await store.load()
    return store
}

private func pausedTick() -> Hearth_V1_SubscribeResponse {
    var bot = Fixtures.usageSummary(month: "").bots[0]
    bot.paused = true
    bot.totals.costMicros = 20_400_000
    return .with {
        $0.event = .usageTick(
            .with {
                $0.bot = bot
                $0.org = Fixtures.usageSummary(month: "").org
                $0.month = Fixtures.currentMonth
            })
    }
}

@MainActor
@Test func thisMonthAndLastMonthComeBackFromTheFixture() async {
    let store = await loadedUsage()
    #expect(store.currentMonth == "2026-10")
    #expect(store.months == ["2026-10", "2026-09"])
    #expect(store.usage(of: Fixtures.bot.id)?.totals.costMicros == 18_400_000)
    #expect(store.usage(of: Fixtures.bot.id)?.budgetCents == 2000)
    await store.select(month: "2026-09")
    #expect(store.summary(month: "2026-09")?.org.costMicros == 9_100_000)
    #expect(store.current?.org.costMicros == 21_300_000)
    #expect(
        UsageStore.title(month: "2026-09", locale: Locale(identifier: "en_US"))
            == "September 2026")
}

@MainActor
@Test func aUsageTickMovesTheFiguresAndTheMonthsAreKept() async {
    let store = await loadedUsage()
    for event in Fixtures.memoryEvents {
        if case .usageTick = event.event { await store.handle(event) }
    }
    #expect(store.usage(of: Fixtures.bot.id)?.totals.costMicros == 18_400_000)
    await store.handle(pausedTick())
    #expect(store.usage(of: Fixtures.bot.id)?.totals.costMicros == 20_400_000)
    #expect(store.months == ["2026-10", "2026-09"])
}

@MainActor
@Test func aBudgetIsSentAsCentsAndTheCoresSummaryIsShown() async {
    let store = await loadedUsage()
    await store.setBudget(botID: Fixtures.bot.id, cents: 3550)
    #expect(store.usage(of: Fixtures.bot.id)?.budgetCents == 3550)
    #expect(store.current?.orgBudgetCents == 5000)
    await store.setOrgBudget(cents: 7500)
    #expect(store.current?.orgBudgetCents == 7500)
    #expect(store.error == nil)
}

@MainActor
@Test func aPausedBotShowsPausedUntilResumed() async {
    let store = await loadedUsage()
    #expect(!store.isPaused(Fixtures.bot.id))
    await store.handle(pausedTick())
    #expect(store.isPaused(Fixtures.bot.id))
    #expect(!store.isPaused("somebody-else"))
    await store.resume(botID: Fixtures.bot.id)
    #expect(!store.isPaused(Fixtures.bot.id))
}

@MainActor
@Test func theSidebarAndTheComposerReadPausedFromTheUsageSummary() async {
    let client = PreviewClient()
    let relay = EventRelay(client: client)
    let usage = UsageStore(client: client, relay: relay)
    await usage.load()
    let bots = BotListStore(client: client, relay: relay)
    bots.usage = usage
    await bots.load()
    let conversation = ConversationStore(
        client: client, relay: relay, conversationID: Fixtures.dmConversation.id,
        usage: usage, botID: Fixtures.bot.id)
    let dana = Fixtures.bot
    #expect(bots.activity(of: dana) != .paused)
    #expect(conversation.pausedPrompt(botName: "Dana") == nil)

    await usage.handle(pausedTick())
    #expect(bots.activity(of: dana) == .paused)
    #expect(bots.isPaused(dana.id))
    #expect(conversation.pausedPrompt(botName: "Dana") == "Dana is paused")

    await usage.resume(botID: dana.id)
    #expect(bots.activity(of: dana) != .paused)
    #expect(conversation.pausedPrompt(botName: "Dana") == nil)
}

@MainActor
@Test func theInboxOffersResumeOnlyForThePauseAndResumingLiftsIt() async {
    let client = PreviewClient()
    let relay = EventRelay(client: client)
    let usage = UsageStore(client: client, relay: relay)
    await usage.load()
    await usage.handle(pausedTick())
    let inbox = InboxStore(client: client, relay: relay, usage: usage)

    var pause = Fixtures.failure
    pause.kind = .budget
    pause.effect = "Paused: Dana used all of this month's $20."
    var warning = pause
    warning.effect = "Dana has used 80% of this month's $20."
    #expect(inbox.canResume(pause))
    #expect(!inbox.canResume(warning))
    #expect(!inbox.canResume(Fixtures.failure))

    await inbox.handle(.with { $0.event = .inboxChanged(.with { $0.item = pause }) })
    await inbox.resume(pause)
    #expect(!usage.isPaused(Fixtures.bot.id))
    #expect(inbox.state(of: pause) == .closed("Resumed", .neutral))
}
