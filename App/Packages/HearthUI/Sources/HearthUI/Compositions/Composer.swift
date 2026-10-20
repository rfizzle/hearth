// Composer is where the owner writes to a bot: a TextEditor, a Send button,
// and a Stop button while the bot is working. Return sends and Shift-Return
// starts a new line; Command-Period stops. Send is disabled while a run works,
// since the core takes one run per conversation at a time, and while the draft
// is blank. It composes a system TextEditor and Buttons, so keyboard and
// VoiceOver behaviour is theirs; the editor's label names the bot. The draft
// is presentation state and clears once the send succeeds, so a refused
// message is still there to try again. A TextEditor until attachments
// need an NSTextView.

import HearthDesign
import SwiftUI

/// The message field under a conversation.
public struct Composer: View {
    let label: String
    let isWorking: Bool
    let onSend: (String) async -> Bool
    let onStop: () -> Void

    @State private var draft: String
    @ScaledMetric(relativeTo: .body) private var minHeight: CGFloat = 36
    @ScaledMetric(relativeTo: .body) private var maxHeight: CGFloat = 160

    /// A composer labelled `label` ("Message Dana"); `isWorking` while the
    /// conversation's run has not ended.
    public init(
        label: String,
        isWorking: Bool,
        draft: String = "",
        onSend: @escaping (String) async -> Bool,
        onStop: @escaping () -> Void
    ) {
        self.label = label
        self.isWorking = isWorking
        self.onSend = onSend
        self.onStop = onStop
        _draft = State(initialValue: draft)
    }

    public var body: some View {
        HStack(alignment: .bottom, spacing: 8) {
            TextEditor(text: $draft)
                .font(Theme.body)
                .foregroundStyle(Theme.color(.textPrimary))
                .scrollContentBackground(.hidden)
                .frame(minHeight: minHeight, maxHeight: maxHeight)
                .padding(6)
                .background(
                    RoundedRectangle(cornerRadius: 8).fill(Theme.color(.surfaceSecondary))
                )
                .accessibilityLabel(label)
                .onKeyPress(keys: [.return], phases: .down) { press in
                    if press.modifiers.contains(.shift) { return .ignored }
                    send()
                    return .handled
                }
            if Self.showsStop(isWorking: isWorking) {
                Button("Stop", action: onStop)
                    .keyboardShortcut(".", modifiers: .command)
            }
            Button("Send", action: send)
                .buttonStyle(.borderedProminent)
                .disabled(!Self.canSend(draft: draft, isWorking: isWorking))
        }
    }

    private func send() {
        guard Self.canSend(draft: draft, isWorking: isWorking) else { return }
        let sent = draft
        Task {
            // Typing during the call keeps what was typed.
            if await onSend(sent.trimmingCharacters(in: .whitespacesAndNewlines)), draft == sent {
                draft = ""
            }
        }
    }

    /// Whether Send is enabled: nothing is running and the draft is not blank.
    static func canSend(draft: String, isWorking: Bool) -> Bool {
        !isWorking && !draft.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
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
        case working

        var draft: String {
            self == .empty ? "" : "Add a section on running costs."
        }
    }

    /// A composer showing `state`, for the gallery and previews.
    public init(previewState state: PreviewState) {
        self.init(
            label: "Message Dana", isWorking: state == .working, draft: state.draft,
            onSend: { _ in true }, onStop: {})
    }
}

#Preview("Empty") { Composer(previewState: .empty).padding() }
#Preview("Drafted") { Composer(previewState: .drafted).padding() }
#Preview("Working") { Composer(previewState: .working).padding() }
