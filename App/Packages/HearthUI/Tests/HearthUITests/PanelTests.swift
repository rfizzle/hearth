// The panels, the template card and the tier picker declare the states the gallery shows.

import HearthKit
import Testing

@testable import HearthUI

@Test func signInPanelHasNinePreviewStates() {
    #expect(SignInPanel.PreviewState.allCases.count == 9)
    #expect(SignInPanel.PreviewState.addingKey.editing == .anthropic)
    #expect(SignInPanel.PreviewState.signedOut.editing == nil)
    #expect(SignInPanel.PreviewState.failed.failure != nil)
    #expect(SignInPanel.PreviewState.signedOut.failure == nil)
    #expect(SignInPanel.PreviewState.signedOut.accounts.count == 4)
    #expect(SignInPanel.PreviewState.serviceList.adding == .list)
    #expect(SignInPanel.PreviewState.serviceFields.adding == .fields(nil))
    #expect(SignInPanel.PreviewState.addingKey.adding == nil)
    #expect(
        SignInPanel.PreviewState.addedService.accounts.contains {
            $0.provider == .openaiCompatible && !$0.profile.isEmpty
        })
}

@Test func anAddedServiceCardShowsItsOwnNameAndWhereItIs() {
    let account = Fixtures.customService
    #expect(account.name == "Acme gateway")
    #expect(account.gives == "llm.acme.example · acme-large")
    let builtin = Fixtures.accounts[1]
    #expect(builtin.name == "Anthropic")
    #expect(builtin.gives == Hearth_V1_Provider.anthropic.gives)
}

@Test func servicePickerHasFivePreviewStates() {
    #expect(ServicePicker.PreviewState.allCases.count == 5)
    #expect(ServicePicker.PreviewState.closed.stage == nil)
    #expect(ServicePicker.PreviewState.customGateway.stage == .fields(nil))
    #expect(Fixtures.knownServices.count > ServicePicker.searchThreshold)
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
