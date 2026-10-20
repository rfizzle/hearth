// Composer is where the owner writes to a bot: a text field on a pane of
// Liquid Glass with a round Send button beside it, which becomes a round Stop
// button while the bot is working. It is the one custom glass hearth draws:
// the control layer floating over the scrolling transcript, mounted by the
// screen as a bottom bar. Return sends and Shift-Return starts a new line;
// Command-Period stops. Send is disabled while a run works, since the core
// takes one run per conversation at a time, and while the draft is blank. It
// composes a system TextField and Buttons, so keyboard and VoiceOver
// behaviour is theirs; the field's label names the bot. The field is a
// vertical-axis TextField that grows to eight lines, not a TextEditor, which
// has no height of its own and fills the whole bar. The draft is presentation
// state, the screen's when it passes a binding so a suggestion chip can fill
// it and move focus here, and clears once the send succeeds, so a refused message is still
// there to try again. Send and Stop are one round button whose symbol replaces
// itself, so the pointer target stays put. While the conversation waits for
// the owner to answer a card above it, the field is disabled and says so in
// place of its label, and Send stays disabled: the core takes no message
// until the answer. Files dropped anywhere on the bar are handed to the
// screen as URLs and come back as chips in a row inside the glass, above the
// field, each with a remove button; the bar takes a tint stroke while a drag
// is over it. The composer reads nothing from a dropped URL. A one-line caption
// under the chips says why a drop took nothing.
// See docs/design.md#materials, docs/design.md#motion and
// docs/model.md#8-approvals

import HearthDesign
import HearthKit
import SwiftUI

/// The message field under a conversation.
public struct Composer: View {
    let label: String
    let isWorking: Bool
    let waitingPrompt: String?
    let attachments: [Hearth_V1_FileRef]
    let attachmentNotice: String?
    let onDrop: ([URL]) -> Void
    let onRemove: (Hearth_V1_FileRef) -> Void
    let onSend: (String) async -> Bool
    let onStop: () -> Void

    let sharedDraft: Binding<String>?
    let focusToken: Int

    @State private var ownDraft: String
    @State private var isTargeted: Bool
    @FocusState private var isFocused: Bool
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    /// A composer labelled `label` ("Message Dana"); `isWorking` while the
    /// conversation's run has not ended, and `waitingPrompt` ("Answer Dana
    /// above first") while it waits for the owner. `attachments` are the
    /// chips for the files dropped and not yet sent; `onDrop` takes what was
    /// dropped on the bar and `onRemove` a chip taken off. A `text` binding
    /// makes the draft the caller's, so a chip elsewhere can fill it; the
    /// field takes focus whenever `focusToken` changes.
    public init(
        label: String,
        isWorking: Bool,
        waitingPrompt: String? = nil,
        attachments: [Hearth_V1_FileRef] = [],
        attachmentNotice: String? = nil,
        onDrop: @escaping ([URL]) -> Void = { _ in },
        onRemove: @escaping (Hearth_V1_FileRef) -> Void = { _ in },
        draft: String = "",
        text: Binding<String>? = nil,
        focusToken: Int = 0,
        isTargeted: Bool = false,
        onSend: @escaping (String) async -> Bool,
        onStop: @escaping () -> Void
    ) {
        self.label = label
        self.isWorking = isWorking
        self.waitingPrompt = waitingPrompt
        self.attachments = attachments
        self.attachmentNotice = attachmentNotice
        self.onDrop = onDrop
        self.onRemove = onRemove
        self.onSend = onSend
        self.onStop = onStop
        sharedDraft = text
        self.focusToken = focusToken
        _ownDraft = State(initialValue: draft)
        _isTargeted = State(initialValue: isTargeted)
    }

    private var draft: Binding<String> { sharedDraft ?? $ownDraft }

    public var body: some View {
        let stops = Self.showsStop(isWorking: isWorking)
        return GlassEffectContainer(spacing: Theme.Spacing.sm) {
            HStack(alignment: .bottom, spacing: Theme.Spacing.sm) {
                editor
                Button(
                    stops ? "Stop" : "Send", systemImage: stops ? "stop.fill" : "arrow.up",
                    action: stops ? onStop : send
                )
                .buttonStyle(.glass)
                .tint(stops ? nil : .accentColor)
                .buttonBorderShape(.circle)
                .labelStyle(.iconOnly)
                .controlSize(.large)
                .contentTransition(.symbolEffect(.replace))
                .keyboardShortcut(stops ? KeyboardShortcut(".", modifiers: .command) : nil)
                .disabled(!stops && !canSend)
                .help(stops ? "Stop (⌘.)" : "Send (Return)")
            }
        }
        .animation(Theme.motion(Theme.quick, reduceMotion: reduceMotion), value: isWorking)
        .animation(
            Theme.motion(Theme.quick, reduceMotion: reduceMotion), value: waitingPrompt != nil
        )
        .animation(
            Theme.motion(Theme.quick, reduceMotion: reduceMotion), value: attachments.map(\.path)
        )
        .dropDestination(for: URL.self) { urls, _ in
            onDrop(urls)
            return !urls.isEmpty
        } isTargeted: {
            isTargeted = $0
        }
        .padding(.horizontal, Theme.Spacing.lg)
        .padding(.bottom, Theme.Spacing.md)
    }

    private var editor: some View {
        VStack(alignment: .leading, spacing: Theme.Spacing.xs) {
            if !attachments.isEmpty {
                chips
            }
            if let attachmentNotice {
                Text(attachmentNotice)
                    .font(Theme.caption)
                    .foregroundStyle(Theme.color(Tone.negative))
            }
            field
        }
        .padding(.horizontal, Theme.Spacing.md)
        .padding(.vertical, Theme.Spacing.sm)
        .glassEffect(.regular, in: .rect(cornerRadius: Theme.Radius.lg))
        .overlay(
            RoundedRectangle(cornerRadius: Theme.Radius.lg, style: .continuous)
                .stroke(Color.accentColor, lineWidth: 2)
                .opacity(isTargeted ? 1 : 0)
        )
    }

    private var chips: some View {
        ScrollView(.horizontal) {
            HStack(spacing: Theme.Spacing.xs) {
                ForEach(attachments, id: \.path) { file in
                    AttachmentChip(file: file) { onRemove(file) }
                }
            }
            .padding(Theme.Spacing.xs)
        }
        .scrollIndicators(.hidden)
    }

    private var field: some View {
        TextField(waitingPrompt ?? label, text: draft, axis: .vertical)
            .textFieldStyle(.plain)
            .focused($isFocused)
            .onChange(of: focusToken) { isFocused = true }
            .lineLimit(1...8)
            .font(Theme.body)
            .foregroundStyle(Theme.color(.textPrimary))
            .accessibilityLabel(waitingPrompt ?? label)
            .disabled(waitingPrompt != nil)
            .submitOnReturn(send)
    }

    private var canSend: Bool {
        Self.canSend(
            draft: draft.wrappedValue, isWorking: isWorking, isWaiting: waitingPrompt != nil)
    }

    private func send() {
        guard canSend else { return }
        let sent = draft.wrappedValue
        Task {
            // Typing during the call keeps what was typed.
            let accepted = await onSend(sent.trimmingCharacters(in: .whitespacesAndNewlines))
            if accepted, draft.wrappedValue == sent { draft.wrappedValue = "" }
        }
    }

    /// Whether Send is enabled: nothing is running or waiting for the owner,
    /// and the draft is not blank.
    static func canSend(draft: String, isWorking: Bool, isWaiting: Bool = false) -> Bool {
        !isWorking && !isWaiting
            && !draft.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
    }

    /// Whether Stop is shown: only while a run works.
    static func showsStop(isWorking: Bool) -> Bool {
        isWorking
    }
}

extension Composer {
    /// The states the gallery and previews show.
    public enum PreviewState: String, CaseIterable, Sendable {
        case empty
        case drafted
        case threeLines
        case working
        case waiting
        case targeted
        case twoChips

        var draft: String {
            switch self {
            case .empty, .waiting, .targeted: ""
            case .drafted, .working, .twoChips: "Add a section on running costs."
            case .threeLines:
                "Add a section on running costs.\nKeep it to one paragraph.\nUse pounds, not dollars."
            }
        }
    }

    /// Two files waiting to be sent.
    static let sampleChips: [Hearth_V1_FileRef] = [
        Fixtures.briefFile,
        .with {
            $0.name = "House survey.pdf"
            $0.path = "/Users/owner/Hearth/Inbox/House survey.pdf"
        },
    ]

    /// A composer showing `state`, for the gallery and previews.
    public init(previewState state: PreviewState) {
        self.init(
            label: "Message Dana", isWorking: state == .working,
            waitingPrompt: state == .waiting ? "Answer Dana above first" : nil,
            attachments: state == .twoChips ? Self.sampleChips : [], draft: state.draft,
            isTargeted: state == .targeted,
            onSend: { _ in true }, onStop: {})
    }
}

#Preview("Empty") { Composer(previewState: .empty).padding() }
#Preview("Drafted") { Composer(previewState: .drafted).padding() }
#Preview("Three lines") { Composer(previewState: .threeLines).padding() }
#Preview("Working") { Composer(previewState: .working).padding() }
#Preview("Targeted") { Composer(previewState: .targeted).padding() }
#Preview("Two chips") { Composer(previewState: .twoChips).padding() }
#Preview("Waiting") { Composer(previewState: .waiting).padding() }
