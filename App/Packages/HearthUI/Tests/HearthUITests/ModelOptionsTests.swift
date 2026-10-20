// The words and the stored form of a model option, the service picker's
// detected row, and what a service edit sends.

import HearthKit
import Testing

@testable import HearthUI

@Test func aPinnedModelIsTheProviderTheServiceAndTheModel() {
    let option = Hearth_V1_ModelOption.with {
        $0.provider = .openaiCompatible
        $0.profile = "ollama"
        $0.model = "llama3:8b"
        $0.label = "Ollama (local)"
    }
    #expect(option.pinned == "openai-compatible:ollama:llama3:8b")
    #expect(option.title == "Ollama (local) · llama3:8b")
    #expect(Hearth_V1_ModelOption(pinned: option.pinned)?.model == "llama3:8b")
    #expect(Hearth_V1_ModelOption(pinned: "anthropic::claude-opus-4-5")?.profile == "")
    #expect(Hearth_V1_ModelOption(pinned: "claude-opus-4-5") == nil)
    #expect(Hearth_V1_ModelOption(pinned: "anthropic::") == nil)
}

@Test func optionsGroupUnderTheirLabelsInOrder() {
    let groups = Hearth_V1_ModelOption.grouped(Fixtures.modelOptions)
    #expect(groups.map(\.label) == ["ChatGPT", "Anthropic", "Ollama (local)"])
    #expect(groups.map(\.options.count) == [2, 2, 1])
}

@Test func aRunningLocalRuntimeIsFirstInThePickerWithItsModelCount() {
    let ordered = ServicePicker.ordered(Fixtures.knownServices)
    #expect(ordered.first?.id == "ollama")
    #expect(ordered.count == Fixtures.knownServices.count)
    #expect(ordered[1].id == "zai")
    #expect(ServicePicker.detail(ordered[0]) == "3 models")
    #expect(ServicePicker.detail(ordered[1]) == nil)
}

@Test func editingAServiceWithAnEmptyKeySendsAnEmptyKeyAndItsProfile() {
    let request = ServiceFields.request(
        profile: "a1b2c3d4", name: "Acme gateway", address: "https://llm.acme.example/v2",
        key: "", model: "acme-large")
    #expect(request.key.isEmpty)
    #expect(request.profile == "a1b2c3d4")
    #expect(request.provider == .openaiCompatible)
    #expect(request.baseURL == "https://llm.acme.example/v2")
}

@Test func theProviderCardHasThreePreviewStates() {
    #expect(ProviderCard.PreviewState.allCases.count == 3)
    #expect(ProviderCard.PreviewState.addedService.isEditing == false)
    #expect(ProviderCard.PreviewState.editing.failure == nil)
    #expect(ProviderCard.PreviewState.refused.failure?.contains("paste the key again") == true)
}
