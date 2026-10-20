// The panels, the template card and the tier picker declare the states the gallery shows.

import Foundation
import HearthDesign
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

@Test func aProjectHeaderSaysProgressAndTheDate() throws {
    let utc = try #require(TimeZone(identifier: "UTC"))
    let today = Date(timeIntervalSince1970: 1_792_486_800)  // 2026-10-20
    let yellow = ProjectHeader.PreviewState.yellow.project
    #expect(ProjectHeader.progress(yellow) == "3 of 7 done")
    #expect(ProjectHeader.progress(ProjectHeader.PreviewState.noTasks.project) == "No tasks yet")
    #expect(ProjectHeader.progress(ProjectHeader.PreviewState.allDone.project) == "All done")

    let due = ProjectHeader.dateLine(of: yellow, today: today, timeZone: utc)
    #expect(due?.text == "Due 20 Nov")
    #expect(due?.tone == .neutral)

    let late = ProjectHeader.dateLine(
        of: ProjectHeader.PreviewState.overdue.project, today: today, timeZone: utc)
    #expect(late?.text == "Was due 30 Sep")
    #expect(late?.tone == .attention)

    let done = ProjectHeader.dateLine(
        of: ProjectHeader.PreviewState.done.project, today: today, timeZone: utc)
    #expect(done?.text == "Done 20 Oct")
    #expect(done?.tone == .positive)

    #expect(
        ProjectHeader.dateLine(
            of: ProjectHeader.PreviewState.noTasks.project, today: today, timeZone: utc) == nil)
}

@Test func theHeadersLightFollowsTheCoreAsTheRowsDoes() {
    let tones = ProjectHeader.PreviewState.allCases.map { ProjectRow.tone(of: $0.project) }
    #expect(
        tones == [.positive, .attention, .negative, .attention, .positive, nil, nil, .attention])
}
