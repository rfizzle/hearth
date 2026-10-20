// Store tests for the tiers and the Advanced pin, against PreviewClient's
// tiers and options and a scripted tier change.

import Foundation
import HearthKit
import HearthUI
import Testing

@testable import HearthFeatures

@MainActor
private func modelsStore() -> ModelsStore {
    let client = PreviewClient()
    return ModelsStore(client: client, relay: EventRelay(client: client))
}

@MainActor
private func botModelStore(override: String = "") -> (BotModelStore, ModelsStore) {
    let models = modelsStore()
    var bot = Fixtures.bot
    bot.modelOverride = override
    return (BotModelStore(client: PreviewClient(), models: models, bot: bot), models)
}

@MainActor
@Test func theTiersAndTheOptionsComeFromTheFixture() async {
    let store = modelsStore()
    await store.load()
    #expect(store.tiers == Fixtures.tierChoices)
    #expect(store.options == Fixtures.modelOptions)
    #expect(store.selection(for: .balanced)?.model == "gpt-5")
    #expect(store.selection(for: .best)?.title == "Anthropic · claude-opus-4-5")
}

@MainActor
@Test func settingATierSendsTheChoiceAndKeepsTheOthers() async throws {
    let store = modelsStore()
    await store.load()
    let local = try #require(store.options.first { $0.profile == "ollama" })
    await store.set(tier: .fast, option: local)

    let fast = try #require(store.tiers.first { $0.tier == .fast })
    #expect(fast.provider == .openaiCompatible)
    #expect(fast.profile == "ollama")
    #expect(fast.model == "llama3.2")
    #expect(store.tiers.count == 3)
    #expect(store.tiers.first { $0.tier == .best } == Fixtures.tierChoices[2])
    #expect(store.selection(for: .fast) == local)
}

@MainActor
@Test func aTierChangeFromTheCoreReplacesTheMapping() async {
    let store = modelsStore()
    await store.load()
    var moved = Fixtures.tierChoices
    moved[1].model = "gpt-5-mini"
    await store.handle(.with { $0.event = .tiersChanged(.with { $0.tiers = moved }) })
    #expect(store.selection(for: .balanced)?.model == "gpt-5-mini")
}

@MainActor
@Test func aTierOnAModelNoLongerOfferedStillShowsIt() async {
    let store = modelsStore()
    await store.load()
    var gone = Fixtures.tierChoices
    gone[0].model = "retired-model"
    await store.handle(.with { $0.event = .tiersChanged(.with { $0.tiers = gone }) })
    #expect(store.selection(for: .fast)?.model == "retired-model")
}

@MainActor
@Test func pinningAModelRoundTripsThroughUpdateBot() async throws {
    let (store, _) = botModelStore()
    await store.load()
    #expect(!store.hasChanged)
    let claude = try #require(store.options.first { $0.model == "claude-sonnet-4-5" })
    store.pinned = claude.pinned
    #expect(store.hasChanged)
    #expect(store.request.modelOverride == "anthropic::claude-sonnet-4-5")
    #expect(!store.request.clearModelOverride_p)

    await store.save()
    #expect(store.didSave)
    #expect(store.error == nil)
    #expect(!store.hasChanged)
    #expect(store.pinned == "anthropic::claude-sonnet-4-5")
}

@MainActor
@Test func clearingThePinSendsTheFlagNotAnEmptyModel() async {
    let (store, _) = botModelStore(override: "anthropic::claude-sonnet-4-5")
    store.pinned = ""
    #expect(store.hasChanged)
    #expect(store.request.clearModelOverride_p)
    #expect(store.request.modelOverride.isEmpty)
    await store.save()
    #expect(store.didSave)
    #expect(store.pinned.isEmpty)
}

@MainActor
@Test func savingWithoutChangingThePinSendsNothing() async {
    let (store, _) = botModelStore(override: "chatgpt::gpt-5")
    await store.save()
    #expect(store.didSave)
    #expect(store.pinned == "chatgpt::gpt-5")
}

@MainActor
@Test func aPinOnAServiceNoLongerOfferedIsStillAmongTheOptions() async {
    let (store, _) = botModelStore(override: "openai-compatible:gone:llama3:8b")
    await store.load()
    let own = store.options.first { $0.profile == "gone" }
    #expect(own?.model == "llama3:8b")
}

@MainActor
@Test func cancellingPutsBackThePin() {
    let (store, _) = botModelStore()
    store.pinned = "chatgpt::gpt-5"
    store.discard()
    #expect(store.pinned.isEmpty)
    #expect(!store.hasChanged)
}
