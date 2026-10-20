// RunStatusLine says where the conversation's run is, under the transcript:
// working (with a small system spinner), waiting for the owner, stopped, or
// failed with the core's plain-words reason in the negative tone. A finished
// or absent run shows nothing. The failure carries a warning symbol as well as
// its text, so it never relies on a tint; the line reads as one element.
// See docs/design.md#the-transcript

import HearthDesign
import HearthKit
import SwiftUI

/// The run's state in a line.
public struct RunStatusLine: View {
    let run: Hearth_V1_Run?
    let botName: String

    public init(run: Hearth_V1_Run?, botName: String) {
        self.run = run
        self.botName = botName
    }

    public var body: some View {
        if let text = Self.text(run: run, botName: botName) {
            HStack(spacing: Theme.Spacing.xs) {
                switch run?.state {
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
            .foregroundStyle(Theme.color(run?.state == .failed ? Tone.negative : Tone.neutral))
            .frame(maxWidth: .infinity, alignment: .leading)
            .accessibilityElement(children: .combine)
        }
    }

    /// What the line says for `run`, or nil when it shows nothing.
    static func text(run: Hearth_V1_Run?, botName: String) -> String? {
        switch run?.state {
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
        case stopped
        case failed
        case done

        var run: Hearth_V1_Run {
            var run = self == .failed ? Fixtures.failedRun : Fixtures.workingRun
            switch self {
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
#Preview("Stopped") { RunStatusLine(previewState: .stopped).padding() }
#Preview("Failed") { RunStatusLine(previewState: .failed).padding() }
#Preview("Done") { RunStatusLine(previewState: .done).padding() }
