// ContextMeter is a quiet gauge for the handbook: a thin progress bar over
// "18 KB of 24 KB" and the plain reason it matters. The bar and the number
// stay neutral until the pages take more than nine tenths of the room, turn
// the attention tone above that, and the negative tone at the cap, where the
// core refuses more. The words carry the meaning; the colour only supports.
// See docs/design.md#colour

import HearthDesign
import HearthKit
import SwiftUI

/// How much of the handbook's room the pages take.
public struct ContextMeter: View {
    let used: Int64
    let cap: Int64

    public init(used: Int64, cap: Int64) {
        self.used = used
        self.cap = cap
    }

    /// Neutral under 90%, attention above, negative at the cap.
    var tone: Tone {
        guard cap > 0 else { return .neutral }
        if used >= cap { return .negative }
        return Double(used) / Double(cap) > 0.9 ? .attention : .neutral
    }

    public var body: some View {
        VStack(alignment: .leading, spacing: Theme.Spacing.xs) {
            ProgressView(value: Double(min(max(used, 0), max(cap, 1))), total: Double(max(cap, 1)))
                .tint(tone == .neutral ? nil : Theme.color(tone))
            Text(caption)
                .font(Theme.caption)
                .foregroundStyle(Theme.color(tone))
                .monospacedDigit()
        }
        .accessibilityElement(children: .combine)
    }

    private var caption: String {
        let amount = "\(size(used)) of \(size(cap))"
        let sentence =
            tone == .negative
            ? "full; shorten a page or remove one"
            : "everything here goes into every run"
        return "\(amount) · \(sentence)"
    }

    private func size(_ bytes: Int64) -> String { bytes.roomText }
}

extension Int64 {
    /// A size of handbook text as the owner reads it: "412 bytes", "18 KB",
    /// "22.5 KB". The core counts in 1024s.
    var roomText: String {
        guard self >= 1024 else { return "\(self) bytes" }
        let kilobytes = Double(self) / 1024
        return "\(kilobytes.formatted(.number.precision(.fractionLength(0...1)))) KB"
    }
}

extension ContextMeter {
    /// The states the gallery and previews show.
    public enum PreviewState: String, CaseIterable, Sendable {
        case roomy
        case nearlyFull
        case full

        var used: Int64 {
            switch self {
            case .roomy: 18 * 1024
            case .nearlyFull: 22 * 1024 + 512
            case .full: 24 * 1024
            }
        }
    }

    /// A meter showing `state`, for the gallery and previews.
    public init(previewState state: PreviewState) {
        self.init(used: state.used, cap: Fixtures.handbookMeter.capBytes)
    }
}

#Preview("Roomy") { ContextMeter(previewState: .roomy).padding() }
#Preview("Nearly full") { ContextMeter(previewState: .nearlyFull).padding() }
#Preview("Full") { ContextMeter(previewState: .full).padding() }
