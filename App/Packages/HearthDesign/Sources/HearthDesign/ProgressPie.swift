// ProgressPie is how far along something is, the way Things draws a project:
// a hairline ring in the secondary style with a wedge of the accent filling
// clockwise from twelve o'clock for done over total, a full disc with a check
// once everything is done, and the empty ring when nothing is done or there is
// nothing to do yet. It is custom drawing because no system control or symbol
// draws an arbitrary fraction as a pie; the full state is the system's
// "checkmark.circle.fill" symbol. It scales with Dynamic Type from 16 pt
// through one scaled metric and carries no number; VoiceOver reads it as one
// element whose value says the count in words ("3 of 7 done", "No tasks yet",
// "All done").
// See docs/design.md#colour

import SwiftUI

/// A small pie of done over total.
public struct ProgressPie: View {
    let done: Int
    let total: Int

    @ScaledMetric(relativeTo: .body) private var size = Theme.Size.progressPie

    /// A pie of `done` out of `total`; a total of zero draws the empty ring.
    public init(done: Int, total: Int) {
        self.done = done
        self.total = total
    }

    public var body: some View {
        Group {
            if Self.isComplete(done: done, total: total) {
                Image(systemName: "checkmark.circle.fill")
                    .resizable()
                    .symbolRenderingMode(.palette)
                    .foregroundStyle(.white, .tint)
            } else {
                ZStack {
                    Circle().strokeBorder(.secondary, lineWidth: 1)
                    Wedge(fraction: Self.fraction(done: done, total: total))
                        .fill(.tint)
                        .padding(size / 8)
                }
            }
        }
        .frame(width: size, height: size)
        .accessibilityElement()
        .accessibilityLabel("Progress")
        .accessibilityValue(Self.spoken(done: done, total: total))
    }

    /// The share of the pie filled, from 0 to 1.
    static func fraction(done: Int, total: Int) -> Double {
        guard total > 0 else { return 0 }
        return min(max(Double(done) / Double(total), 0), 1)
    }

    /// Whether the pie is the full disc with a check.
    static func isComplete(done: Int, total: Int) -> Bool {
        total > 0 && done >= total
    }

    /// The pie as VoiceOver says it.
    public static func spoken(done: Int, total: Int) -> String {
        if total <= 0 { return "No tasks yet" }
        if done >= total { return "All done" }
        return "\(max(done, 0)) of \(total) done"
    }
}

/// The filled part: from the centre, clockwise from twelve o'clock.
private struct Wedge: Shape {
    let fraction: Double

    func path(in rect: CGRect) -> Path {
        var path = Path()
        guard fraction > 0 else { return path }
        let centre = CGPoint(x: rect.midX, y: rect.midY)
        path.move(to: centre)
        path.addArc(
            center: centre, radius: min(rect.width, rect.height) / 2,
            startAngle: .degrees(-90), endAngle: .degrees(-90 + 360 * fraction),
            clockwise: false)
        path.closeSubpath()
        return path
    }
}

extension ProgressPie {
    /// The states the gallery and previews show.
    public enum PreviewState: String, CaseIterable, Sendable {
        case noTasks = "no tasks"
        case nothingDone = "0 of 7"
        case someDone = "3 of 7"
        case allDone = "7 of 7"

        /// How many are done in this state.
        public var done: Int {
            switch self {
            case .noTasks, .nothingDone: 0
            case .someDone: 3
            case .allDone: 7
            }
        }

        /// How many there are in this state.
        public var total: Int { self == .noTasks ? 0 : 7 }
    }

    /// A pie showing `state`, for the gallery and previews.
    public init(previewState state: PreviewState) {
        self.init(done: state.done, total: state.total)
    }
}

#Preview("No tasks") { ProgressPie(previewState: .noTasks).padding() }
#Preview("0 of 7") { ProgressPie(previewState: .nothingDone).padding() }
#Preview("3 of 7") { ProgressPie(previewState: .someDone).padding() }
#Preview("7 of 7") { ProgressPie(previewState: .allDone).padding() }
