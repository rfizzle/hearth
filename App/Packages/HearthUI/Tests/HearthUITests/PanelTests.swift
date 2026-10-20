// The panels, the template card and the tier picker declare the states the gallery shows.

import Testing

@testable import HearthUI

@Test func signInPanelHasFivePreviewStates() {
    #expect(SignInPanel.PreviewState.allCases.count == 5)
    #expect(SignInPanel.PreviewState.addingKey.editing == .anthropic)
    #expect(SignInPanel.PreviewState.signedOut.editing == nil)
    #expect(SignInPanel.PreviewState.failed.failure != nil)
    #expect(SignInPanel.PreviewState.signedOut.failure == nil)
}

@Test func integrationsPanelHasThreePreviewStates() {
    #expect(IntegrationsPanel.PreviewState.allCases.count == 3)
    #expect(IntegrationsPanel.PreviewState.configured.integrations.allSatisfy { $0.configured })
    #expect(IntegrationsPanel.PreviewState.notConfigured.integrations.allSatisfy { !$0.configured })
}

@Test func templateCardHasThreePreviewStates() {
    #expect(TemplateCard.PreviewState.allCases.count == 3)
    #expect(TemplateCard.PreviewState.describe.role == "Describe the job")
}

@Test func tierPickerHasAStatePerTier() {
    #expect(TierPicker.PreviewState.allCases.map(\.tier) == [.fast, .balanced, .best])
}
