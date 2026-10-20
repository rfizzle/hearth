// DecisionBar is the four answers to an approval in one row: Deny on the
// leading edge, then Allow for This Conversation, Always Allow for the bot and
// Allow Once, the one prominent button, on the trailing edge. On a card too
// narrow for the row it is Deny, a menu holding the two conditional allows,
// and Allow Once. Always Allow is absent where the core says the capability
// can never stand, as for deleting. Deny reveals a one-line reason field with
// Cancel (Escape) and Decline, sliding down in place of the row. It composes
// system Buttons, a Menu and a TextField, so keyboard and VoiceOver behaviour
// is theirs; the row decides nothing and reports the owner's choice. Only the
// first card on a screen binds Allow Once to Return, so two cards never claim
// the same key.
// See docs/model.md#8-approvals and docs/design.md#motion

import HearthDesign
import HearthKit
import SwiftUI

/// The four decisions on an approval.
public struct DecisionBar: View {
    let botName: String
    let canAlwaysAllow: Bool
    let isDeciding: Bool
    let isDefault: Bool
    let onDecide: (Hearth_V1_Decision, String) -> Void

    @State private var isDenying: Bool
    @State private var reason = ""
    @FocusState private var reasonFocused: Bool
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    /// A bar for an approval `botName` asked for; `canAlwaysAllow` offers the
    /// standing grant, `isDeciding` while the core answers, `isDefault` binds
    /// Allow Once to Return. `onDecide` gets the decision and the reason,
    /// empty unless the owner declined with one.
    public init(
        botName: String,
        canAlwaysAllow: Bool,
        isDeciding: Bool = false,
        isDefault: Bool = true,
        isDenying: Bool = false,
        onDecide: @escaping (Hearth_V1_Decision, String) -> Void
    ) {
        self.botName = botName
        self.canAlwaysAllow = canAlwaysAllow
        self.isDeciding = isDeciding
        self.isDefault = isDefault
        self.onDecide = onDecide
        _isDenying = State(initialValue: isDenying)
    }

    public var body: some View {
        VStack(alignment: .leading, spacing: Theme.Spacing.sm) {
            if isDenying {
                denyRow
                    .transition(.move(edge: .top).combined(with: .opacity))
            } else {
                ViewThatFits(in: .horizontal) {
                    wide
                    narrow
                }
            }
        }
        .disabled(isDeciding)
        .animation(Theme.motion(Theme.move, reduceMotion: reduceMotion), value: isDenying)
        .accessibilityElement(children: .contain)
        .accessibilityLabel("Decision")
    }

    private var wide: some View {
        HStack(spacing: Theme.Spacing.sm) {
            denyButton
            Spacer(minLength: Theme.Spacing.sm)
            progress
            forConversationButton
            if canAlwaysAllow {
                alwaysButton
            }
            allowOnceButton
        }
    }

    private var narrow: some View {
        HStack(spacing: Theme.Spacing.sm) {
            denyButton
            Spacer(minLength: Theme.Spacing.sm)
            progress
            Menu("Allow…") {
                forConversationButton
                if canAlwaysAllow {
                    alwaysButton
                }
            }
            .fixedSize()
            .help("More ways to allow this")
            allowOnceButton
        }
    }

    private var denyRow: some View {
        HStack(spacing: Theme.Spacing.sm) {
            TextField("Reason (optional)", text: $reason)
                .textFieldStyle(.roundedBorder)
                .font(Theme.body)
                .focused($reasonFocused)
                .onSubmit(decline)
                .accessibilityLabel("Reason for declining, optional")
            progress
            Button("Cancel") {
                isDenying = false
                reason = ""
            }
            .keyboardShortcut(.cancelAction)
            Button("Decline", action: decline)
        }
        .onAppear { reasonFocused = true }
    }

    @ViewBuilder
    private var progress: some View {
        if isDeciding {
            ProgressView()
                .controlSize(.small)
                .accessibilityLabel("Sending your answer")
        }
    }

    private var denyButton: some View {
        Button("Deny") { isDenying = true }
            .help("Don't let \(botName) do this")
    }

    private var forConversationButton: some View {
        Button("Allow for This Conversation") { onDecide(.allowForTask, "") }
            .help("Allow this kind of thing until the conversation ends")
    }

    private var alwaysButton: some View {
        Button("Always Allow for \(botName)") { onDecide(.alwaysAllow, "") }
            .help("\(botName) won't ask about this again")
    }

    private var allowOnceButton: some View {
        Button("Allow Once") { onDecide(.allowOnce, "") }
            .buttonStyle(.borderedProminent)
            .keyboardShortcut(isDefault ? KeyboardShortcut.defaultAction : nil)
            .help("Allow this one time")
    }

    private func decline() {
        onDecide(.deny, Self.reason(reason))
    }

    /// The reason as sent: what was typed, trimmed.
    static func reason(_ typed: String) -> String {
        typed.trimmingCharacters(in: .whitespacesAndNewlines)
    }
}

extension DecisionBar {
    /// The states the gallery and previews show.
    public enum PreviewState: String, CaseIterable, Sendable {
        case wide
        case alwaysAllowOffered
        case narrow
        case denying
        case deciding

        /// The width the state is drawn at: narrow enough for the menu, or
        /// wide enough for the row.
        public var width: CGFloat {
            self == .narrow ? 320 : 600
        }
    }

    /// A bar showing `state`, for the gallery and previews.
    public init(previewState state: PreviewState) {
        self.init(
            botName: Fixtures.bot.name,
            canAlwaysAllow: state == .alwaysAllowOffered || state == .narrow,
            isDeciding: state == .deciding,
            isDenying: state == .denying,
            onDecide: { _, _ in })
    }
}

#Preview("Wide") { DecisionBar(previewState: .wide).frame(width: 600).padding() }
#Preview("Always Allow offered") {
    DecisionBar(previewState: .alwaysAllowOffered).frame(width: 600).padding()
}
#Preview("Narrow") { DecisionBar(previewState: .narrow).frame(width: 320).padding() }
#Preview("Denying") { DecisionBar(previewState: .denying).frame(width: 600).padding() }
#Preview("Deciding") { DecisionBar(previewState: .deciding).frame(width: 600).padding() }
