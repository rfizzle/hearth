// The transcript's rows, the status line and the composer: what each shows
// for the fixture states the gallery walks.

import HearthDesign
import HearthKit
import Testing

@testable import HearthUI

@MainActor
@Test func theComposerDisablesSendAndShowsStopWhileARunWorks() {
    #expect(!Composer.canSend(draft: "Add a section.", isWorking: true))
    #expect(Composer.showsStop(isWorking: true))
    #expect(Composer.canSend(draft: "Add a section.", isWorking: false))
    #expect(!Composer.showsStop(isWorking: false))
    #expect(!Composer.canSend(draft: " \n", isWorking: false))
    #expect(Composer.PreviewState.allCases.count == 5)
    #expect(Composer.PreviewState.threeLines.draft.split(separator: "\n").count == 3)
    #expect(Composer.PreviewState.empty.draft.isEmpty)
}

@MainActor
@Test func theStatusLineSaysWorkingStoppedAndTheFailure() {
    let name = Fixtures.bot.name
    #expect(RunStatusLine.text(run: Fixtures.workingRun, botName: name) == "Dana is working…")
    #expect(RunStatusLine.PreviewState.stopped.run.state == .cancelled)
    #expect(
        RunStatusLine.text(run: RunStatusLine.PreviewState.stopped.run, botName: name) == "Stopped")
    #expect(
        RunStatusLine.text(run: Fixtures.failedRun, botName: name) == Fixtures.failedRun.failure)
    #expect(RunStatusLine.text(run: RunStatusLine.PreviewState.done.run, botName: name) == nil)
    #expect(RunStatusLine.text(run: nil, botName: name) == nil)
}

@MainActor
@Test func completeMarkdownIsParsedAndBrokenMarkdownIsKept() {
    let parsed = MessageRow.markdown("I've written **the brief**.")
    #expect(String(parsed.characters) == "I've written the brief.")
    #expect(String(MessageRow.markdown("a [b").characters) == "a [b")
}

@MainActor
@Test func theMessageStatesCoverOwnerStreamingCompleteFileAndSystem() {
    let states = MessageRow.PreviewState.self
    #expect(MessageRow(previewState: .owner).isOwner)
    #expect(!MessageRow(previewState: .streaming).isOwner)
    #expect(!MessageRow(previewState: .ownerLong).message.streaming)
    #expect(MessageRow(previewState: .ownerLong).isOwner)
    #expect(states.ownerLong.message.content.count > states.owner.message.content.count)
    #expect(states.streaming.message.streaming)
    #expect(!states.complete.message.streaming)
    #expect(states.withFile.message.files == [Fixtures.briefFile])
    #expect(states.system.message.kind == .system)
}

@MainActor
@Test func theToolRowSummarisesAndShowsTheFileTheResultWrote() {
    #expect(ToolCallRow.PreviewState.running.messages.result == nil)
    #expect(ToolCallRow.PreviewState.wroteAFile.messages.result?.files == [Fixtures.briefFile])
    let call = ToolCallRow.PreviewState.done.messages.call.tool
    #expect(ToolCallRow.summary(call) == call.summary)
    #expect(ToolCallRow.summary(.with { $0.name = "web.fetch" }) == "web.fetch")
    #expect(ToolCallRow.PreviewState.runningExpanded.messages.result == nil)
    #expect(FileRow.PreviewState.allCases.count == 2)
}

@MainActor
@Test func aBotAvatarIsASymbolACharacterOrInitials() {
    #expect(Avatar.content(name: "Dana", avatar: "magnifyingglass") == .symbol("magnifyingglass"))
    #expect(Avatar.content(name: "Dana", avatar: "🦊") == .character("🦊"))
    #expect(Avatar.content(name: "Dana", avatar: "") == .initials("Dana"))
}

@MainActor
@Test func eachToolHasItsSymbolAndTheRestShareOne() {
    #expect(ToolCallRow.symbol(for: "web.search") == "magnifyingglass")
    #expect(ToolCallRow.symbol(for: "web.fetch") == "globe")
    #expect(ToolCallRow.symbol(for: "files.write") == "doc.badge.plus")
    #expect(ToolCallRow.symbol(for: "files.read") == "doc.text")
    #expect(ToolCallRow.symbol(for: "calendar.add") == "gearshape")
}
