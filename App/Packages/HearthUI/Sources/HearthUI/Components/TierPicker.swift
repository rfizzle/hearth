// TierPicker chooses how much thinking a bot does: Fast, Balanced or Best. It
// wraps a segmented Picker, so keyboard and VoiceOver behaviour is the
// system's, and a line under it says what the choice means in plain words.
// The picker edits a binding and decides nothing.

import HearthDesign
import HearthKit
import SwiftUI

/// The quality tier choice.
public struct TierPicker: View {
    @Binding var tier: Hearth_V1_Tier

    public init(tier: Binding<Hearth_V1_Tier>) {
        _tier = tier
    }

    public var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            Picker("Quality", selection: $tier) {
                ForEach(Self.choices, id: \.self) { choice in
                    Text(Self.name(choice)).tag(choice)
                }
            }
            .pickerStyle(.segmented)
            Text(Self.hint(tier))
                .font(Theme.body)
                .foregroundStyle(Theme.color(.textSecondary))
        }
    }

    static let choices: [Hearth_V1_Tier] = [.fast, .balanced, .best]

    static func name(_ tier: Hearth_V1_Tier) -> String {
        switch tier {
        case .fast: "Fast"
        case .best: "Best"
        default: "Balanced"
        }
    }

    static func hint(_ tier: Hearth_V1_Tier) -> String {
        switch tier {
        case .fast: "Quick answers for simple jobs."
        case .best: "The most careful work, and the slowest."
        default: "A good mix of speed and care."
        }
    }
}

extension TierPicker {
    /// The states the gallery and previews show.
    public enum PreviewState: String, CaseIterable, Sendable {
        case fast
        case balanced
        case best

        var tier: Hearth_V1_Tier {
            switch self {
            case .fast: .fast
            case .balanced: .balanced
            case .best: .best
            }
        }
    }

    /// A picker showing `state`, for the gallery and previews.
    public init(previewState state: PreviewState) {
        self.init(tier: .constant(state.tier))
    }
}

#Preview("Fast") { TierPicker(previewState: .fast).padding() }
#Preview("Balanced") { TierPicker(previewState: .balanced).padding() }
#Preview("Best") { TierPicker(previewState: .best).padding() }
