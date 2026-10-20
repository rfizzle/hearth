// RunStatusLine says where the conversation's run is, under the transcript:
// working (with a small system spinner), waiting for the owner in the
// attention tone, stopped, or failed with the core's plain-words reason in the
// negative tone. A finished or absent run shows nothing. The failure and the
// wait each carry a symbol as well as their text, so neither relies on a
// tint; the line reads as one element. An item open for the owner reads as
// waiting whatever the run last reported, since a resync forgets the run.
// See docs/design.md#the-transcript and docs/model.md#8-approvals

import HearthDesign
import HearthKit
import SwiftUI

/// The run's state in a line.
public struct RunStatusLine: View {
    let run: Hearth_V1_Run?
    let botName: String
    let isWaiting: Bool

    /// A line for `run`; `isWaiting` while an item is open for the owner.
    public init(run: Hearth_V1_Run?, botName: String, isWaiting: Bool = false) {
        self.run = run
        self.botName = botName
        self.isWaiting = isWaiting
    }

    public var body: some View {
        if let text = Self.text(run: run, botName: botName, isWaiting: isWaiting) {
            HStack(spacing: Theme.Spacing.xs) {
                switch Self.waits(run, isWaiting) ? .waitingOwner : run?.state {
                case .waitingOwner:
                    Image(systemName: "hand.raised")
                case .queued, .working:
                    ProgressView().controlSize(.small)
                case .failed:
                    Image(systemName: "exclamationmark.triangle")
                default:
                    EmptyView()
                }
                Text(text)
            }
            .font(Theme.caption)
            .foregroundStyle(Theme.color(Self.tone(run: run, isWaiting: isWaiting)))
            .frame(maxWidth: .infinity, alignment: .leading)
            .accessibilityElement(children: .combine)
        }
    }

    /// The tone of the line: attention while waiting, negative on a failure.
    static func tone(run: Hearth_V1_Run?, isWaiting: Bool = false) -> Tone {
        if Self.waits(run, isWaiting) { return .attention }
        return run?.state == .failed ? .negative : .neutral
    }

    private static func waits(_ run: Hearth_V1_Run?, _ isWaiting: Bool) -> Bool {
        isWaiting || run?.state == .waitingOwner
    }

    /// What the line says for `run`, or nil when it shows nothing.
    static func text(run: Hearth_V1_Run?, botName: String, isWaiting: Bool = false) -> String? {
        if isWaiting { return "\(botName) is waiting for you" }
        return switch run?.state {
        case .queued, .working:
            "\(botName) is working…"
        case .waitingOwner:
            "\(botName) is waiting for you"
        case .cancelled:
            "Stopped"
        case .failed:
            run.map(\.failure).flatMap { $0.isEmpty ? nil : $0 } ?? "\(botName) couldn't finish."
        default:
            nil
        }
    }
}

extension RunStatusLine {
    /// The states the gallery and previews show.
    public enum PreviewState: String, CaseIterable, Sendable {
        case working
        case waiting
        case stopped
        case failed
        case done

        var run: Hearth_V1_Run {
            var run = self == .failed ? Fixtures.failedRun : Fixtures.workingRun
            switch self {
            case .waiting: run.state = .waitingOwner
            case .stopped: run.state = .cancelled
            case .done: run.state = .done
            case .working, .failed: break
            }
            return run
        }
    }

    /// A line showing `state`, for the gallery and previews.
    public init(previewState state: PreviewState) {
        self.init(run: state.run, botName: Fixtures.bot.name)
    }
}

#Preview("Working") { RunStatusLine(previewState: .working).padding() }
#Preview("Waiting") { RunStatusLine(previewState: .waiting).padding() }
#Preview("Stopped") { RunStatusLine(previewState: .stopped).padding() }
#Preview("Failed") { RunStatusLine(previewState: .failed).padding() }
#Preview("Done") { RunStatusLine(previewState: .done).padding() }
