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
// state and clears once the send succeeds, so a refused message is still
// there to try again. Send and Stop are one round button whose symbol replaces
// itself, so the pointer target stays put. While the conversation waits for
// the owner to answer a card above it, the field is disabled and says so in
// place of its label, and Send stays disabled: the core takes no message
// until the answer.
// See docs/design.md#materials, docs/design.md#motion and
// docs/model.md#8-approvals

import HearthDesign
import SwiftUI

/// The message field under a conversation.
public struct Composer: View {
    let label: String
    let isWorking: Bool
    let waitingPrompt: String?
    let onSend: (String) async -> Bool
    let onStop: () -> Void

    @State private var draft: String
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    /// A composer labelled `label` ("Message Dana"); `isWorking` while the
    /// conversation's run has not ended, and `waitingPrompt` ("Answer Dana
    /// above first") while it waits for the owner.
    public init(
        label: String,
        isWorking: Bool,
        waitingPrompt: String? = nil,
        draft: String = "",
        onSend: @escaping (String) async -> Bool,
        onStop: @escaping () -> Void
    ) {
        self.label = label
        self.isWorking = isWorking
        self.waitingPrompt = waitingPrompt
        self.onSend = onSend
        self.onStop = onStop
        _draft = State(initialValue: draft)
    }

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
        .padding(.horizontal, Theme.Spacing.lg)
        .padding(.bottom, Theme.Spacing.md)
    }

    private var editor: some View {
        TextField(waitingPrompt ?? label, text: $draft, axis: .vertical)
            .textFieldStyle(.plain)
            .lineLimit(1...8)
            .font(Theme.body)
            .foregroundStyle(Theme.color(.textPrimary))
            .padding(.horizontal, Theme.Spacing.md)
            .padding(.vertical, Theme.Spacing.sm)
            .glassEffect(.regular, in: .rect(cornerRadius: Theme.Radius.lg))
            .accessibilityLabel(waitingPrompt ?? label)
            .disabled(waitingPrompt != nil)
            .onKeyPress(keys: [.return], phases: .down) { press in
                if press.modifiers.contains(.shift) { return .ignored }
                send()
                return .handled
            }
    }

    private var canSend: Bool {
        Self.canSend(draft: draft, isWorking: isWorking, isWaiting: waitingPrompt != nil)
    }

    private func send() {
        guard canSend else { return }
        let sent = draft
        Task {
            // Typing during the call keeps what was typed.
            if await onSend(sent.trimmingCharacters(in: .whitespacesAndNewlines)), draft == sent {
                draft = ""
            }
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

        var draft: String {
            switch self {
            case .empty, .waiting: ""
            case .drafted, .working: "Add a section on running costs."
            case .threeLines:
                "Add a section on running costs.\nKeep it to one paragraph.\nUse pounds, not dollars."
            }
        }
    }

    /// A composer showing `state`, for the gallery and previews.
    public init(previewState state: PreviewState) {
        self.init(
            label: "Message Dana", isWorking: state == .working,
            waitingPrompt: state == .waiting ? "Answer Dana above first" : nil, draft: state.draft,
            onSend: { _ in true }, onStop: {})
    }
}

#Preview("Empty") { Composer(previewState: .empty).padding() }
#Preview("Drafted") { Composer(previewState: .drafted).padding() }
#Preview("Three lines") { Composer(previewState: .threeLines).padding() }
#Preview("Working") { Composer(previewState: .working).padding() }
#Preview("Waiting") { Composer(previewState: .waiting).padding() }
