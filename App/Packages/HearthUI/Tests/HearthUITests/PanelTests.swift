// The panels, the template card, the tier picker and the deliverable widgets
// declare the states the gallery shows; the deliverable fixtures decode, and a
// bound widget says where its figure came from.

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

@Test func everyShippedDeliverableDecodes() {
    let kinds = Fixtures.shippedDeliverableIDs.map { Fixtures.deliverable($0).kind }
    #expect(kinds == [.report, .dashboard, .report, .report, .report])
    for id in Fixtures.shippedDeliverableIDs {
        let spec = Fixtures.deliverable(id)
        #expect(spec.unsupported == nil)
        #expect(spec.template?.id == id)
        #expect(spec.template?.name?.isEmpty == false)
        #expect(spec.sections.allSatisfy { !$0.widgets.isEmpty && $0.allow?.isEmpty == false })
    }
}

@Test func theKitchenSinkHoldsEveryKindOfWidget() {
    let spec = Fixtures.kitchenSink
    // The case's name is the kind's: "metric(...)" reads as metric.
    let kinds = spec.sections.flatMap(\.widgets).map {
        String(String(describing: $0.content).prefix { $0 != "(" })
    }
    #expect(
        kinds == [
            "metric", "progress", "light", "line", "bar", "pie", "table", "list", "callout", "text",
        ])
    #expect(spec.sections.map(\.columns) == [3, 2, 1, 2])
    #expect(spec.sections[1].widgets[0].span == 2)
    #expect(spec.sections.flatMap(\.widgets).compactMap(\.measure).count == 3)
}

@Test func aDeliverableDecodesStrictly() throws {
    let base = #"{"kind":"report","title":"T","version":1,"sections":[{"widgets":[WIDGET]}]}"#
    func decode(_ widget: String) throws -> Deliverable {
        try Deliverable(json: Data(base.replacingOccurrences(of: "WIDGET", with: widget).utf8))
    }
    #expect(throws: DecodingError.self) {
        try decode(#"{"kind":"text","markdown":"x","colour":"red"}"#)
    }
    #expect(throws: DecodingError.self) { try decode(#"{"kind":"table","measure":"inbox.open"}"#) }
    #expect(throws: DecodingError.self) { try decode(#"{"kind":"text","markdown":null}"#) }
    #expect(throws: DecodingError.self) {
        try decode(#"{"kind":"metric","label":"L","value":"3"}"#)
    }
    let gauge = try decode(#"{"kind":"gauge","needle":3}"#)
    #expect(gauge.sections[0].widgets[0].content == .unsupported(kind: "gauge"))
    #expect(gauge.sections[0].columns == 1)
    #expect(Fixtures.newerDeliverable.unsupported != nil)
    #expect(Fixtures.newerDeliverable.title == "Office next year")
    #expect(Fixtures.newerDeliverable.sections.isEmpty)
}

@Test func aBoundWidgetSaysWhereItsFigureCameFrom() throws {
    let utc = try #require(TimeZone(identifier: "UTC"))
    let metric = Fixtures.kitchenSink.sections[0].widgets[0]
    let written = BoundValue.of(metric, in: nil)
    #expect(written.label(timeZone: utc) == "As written 20 Oct")
    #expect(written.tone == .neutral)
    let live = BoundValue.of(metric, in: Fixtures.liveMeasures)
    #expect(live.label(timeZone: utc) == "Live")
    let missing = BoundValue.of(metric, in: Fixtures.missingMeasures)
    #expect(missing.label(timeZone: utc) == "Not priced")
    #expect(missing.tone == .attention)
    let progress = Fixtures.kitchenSink.sections[0].widgets[1]
    #expect(BoundValue.of(progress, in: Fixtures.missingMeasures).label() == "No such project")
    let chart = Fixtures.kitchenSink.sections[1].widgets[0]
    #expect(BoundValue.of(chart, in: Fixtures.liveMeasures) == .unbound)
    #expect(BoundValue.unbound.label() == nil)
}

@Test func aBoundFigureIsLiveWrittenOrADash() {
    let english = Locale(identifier: "en_US")
    let widget = Fixtures.kitchenSink.sections[0].widgets[0]
    guard case .metric(let metric) = widget.content else {
        Issue.record("the kitchen sink opens with a metric")
        return
    }
    let written = BoundValue.of(widget, in: nil)
    #expect(MetricTile.value(metric, written, locale: english) == "$12.40")
    let live = BoundValue.of(widget, in: Fixtures.liveMeasures)
    #expect(MetricTile.value(metric, live, locale: english) == "$3.45")
    let missing = BoundValue.of(widget, in: Fixtures.missingMeasures)
    #expect(MetricTile.value(metric, missing, locale: english) == "—")
    let count = ProgressCard.count(.init(label: "L", done: 3, total: 7), .unbound)
    #expect(ProgressCard.words(count) == "3 of 7")
    let budget = Hearth_V1_MeasureValue.with {
        $0.kind = .progress
        $0.unit = "cents"
        $0.done = 4500
        $0.total = 10000
    }
    let spent = ProgressCard.count(.init(label: "Budget", done: nil, total: nil), .live(budget))
    #expect(ProgressCard.words(spent, locale: english) == "$45.00 of $100.00")
}

@Test func deliverableNumbersFollowTheirFormat() {
    let english = Locale(identifier: "en_US")
    #expect(DeliverableDisplay.number(1234.5, .currency, locale: english) == "$1,234.50")
    #expect(DeliverableDisplay.number(2.5, .integer, locale: english) == "3")
    #expect(DeliverableDisplay.number(12.26, .percent, locale: english) == "12.3%")
    #expect(DeliverableDisplay.number(1.23456, .number, locale: english) == "1.235")
    #expect(DeliverableDisplay.number(2, .number, decimals: 2, locale: english) == "2.00")
}

@Test func aTextBlockOpensWebAndMailLinksOnly() throws {
    let links = [
        ("https://example.com", true), ("HTTP://example.com", true), ("mailto:a@example.com", true),
        ("javascript:alert(1)", false), ("file:///etc/passwd", false), ("data:text/html,x", false),
    ]
    for (link, opens) in links {
        #expect(DeliverableDisplay.opens(try #require(URL(string: link))) == opens)
    }
}

@Test func aSectionFillsRowsUpToItsColumns() {
    let trends = Fixtures.kitchenSink.sections[1]
    #expect(DeliverableView.rows(trends).map { $0.map(\.span) } == [[2], [1, 1]])
    #expect(DeliverableView.rows(Fixtures.kitchenSink.sections[0]).map(\.count) == [3])
    let row = DeliverableView.rows(Fixtures.deliverable("dashboard").sections[1])[0]
    #expect(DeliverableView.free(row, columns: 2) == 0)
    #expect(ChartCard.hues(8).count == 8)
}

@Test func theDeliverableWidgetsDeclareTheirStates() {
    #expect(DeliverableView.PreviewState.allCases.count == 9)
    #expect(MetricTile.PreviewState.allCases.count == 5)
    #expect(ChartCard.PreviewState.allCases.count == 8)
    #expect(ProgressCard.PreviewState.live.bound.live?.done == 4)
    #expect(LightCard.PreviewState.missing.bound.tone == .attention)
}
