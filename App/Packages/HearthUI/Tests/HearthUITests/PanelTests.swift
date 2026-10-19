// The panels, the template card and the tier picker declare the states the gallery shows.

import Testing

@testable import HearthUI

@Test func signInPanelHasFourPreviewStates() {
    #expect(SignInPanel.PreviewState.allCases.count == 4)
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
