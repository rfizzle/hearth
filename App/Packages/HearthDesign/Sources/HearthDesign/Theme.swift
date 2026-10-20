// Theme turns the tokens in Resources/tokens.json into SwiftUI values by role.
// A colour token carries a light and a dark value; the NSColor behind it
// resolves against the current appearance, so views never branch on it.
// make tokens-check reads the same file and checks every pairing's contrast.
//
// The colour roles are text.primary and text.secondary; surface.primary,
// surface.secondary and surface.card; and tone.positive, tone.attention and
// tone.negative, each with a .fill to sit behind it. Text and surface colours
// are hex pairs rather than the system's label colours because the contrast
// check can measure a hex pair and cannot measure a system colour that carries
// alpha. Anything the system already names (the accent, hairlines, the window
// background) is not a role: use .tint, .separator and .background.
//
// Type is the system's text styles by role, spacing and radii are the scales
// below, and sizes are the few fixed measures the screens share. None of them
// is in the JSON: nothing checks them.
// Motion is two named animations and one helper that honours Reduce Motion.
// See docs/design.md#colour

import AppKit
import SwiftUI

/// A colour named by what it is for, not what it looks like.
public enum ColorRole: String, CaseIterable, Sendable {
    case textPrimary = "text.primary"
    case textSecondary = "text.secondary"
    case surfacePrimary = "surface.primary"
    /// A quieter surface set on the primary one: an avatar's disc, the
    /// owner's message.
    case surfaceSecondary = "surface.secondary"
    /// A raised surface for a card set on the window.
    case surfaceCard = "surface.card"
    case tonePositive = "tone.positive"
    case tonePositiveFill = "tone.positive.fill"
    case toneAttention = "tone.attention"
    case toneAttentionFill = "tone.attention.fill"
    case toneNegative = "tone.negative"
    case toneNegativeFill = "tone.negative.fill"
}

/// A status in plain terms; views pick a tone, never a colour.
public enum Tone: CaseIterable, Sendable {
    case neutral
    case positive
    case attention
    case negative
}

/// Tokens by role.
public enum Theme {
    /// The welcome mark only.
    public static let largeTitle: Font = .largeTitle
    /// A screen's or sheet's heading.
    public static let title: Font = .title2
    /// A name, a card title.
    public static let headline: Font = .headline
    /// Body text: the transcript, fields, rows.
    public static let body: Font = .body
    /// A row's second line, a tool summary.
    public static let callout: Font = .callout
    /// Small print beside body text: a time, a status line, a pill.
    public static let caption: Font = .caption
    /// Tool arguments and results.
    public static let monospaced: Font = .body.monospaced()

    /// The six-step spacing scale.
    public enum Spacing {
        public static let xs: CGFloat = 4
        public static let sm: CGFloat = 8
        public static let md: CGFloat = 12
        public static let lg: CGFloat = 16
        public static let xl: CGFloat = 24
        public static let xxl: CGFloat = 32
    }

    /// The three corner radii; a capsule is drawn with `Capsule`.
    public enum Radius {
        public static let sm: CGFloat = 6
        public static let md: CGFloat = 10
        public static let lg: CGFloat = 16
    }

    /// Fixed measures the screens share.
    public enum Size {
        public static let avatar: CGFloat = 28
        public static let avatarLarge: CGFloat = 44
        public static let statusDot: CGFloat = 8
        public static let fileIcon: CGFloat = 20
        public static let minTarget: CGFloat = 24
        public static let readableWidth: CGFloat = 640
        public static let bubbleWidth: CGFloat = 480
        public static let formWidth: CGFloat = 480
        public static let sheetWidth: CGFloat = 560
        public static let sidebarMin: CGFloat = 200
        public static let sidebarIdeal: CGFloat = 240
        public static let sidebarMax: CGFloat = 320
    }

    /// Feedback that follows a click, a swap or a fade.
    public static let quick: Animation = .easeOut(duration: 0.2)

    /// A layout move, where height or position changes: a spring with no
    /// bounce.
    public static let move: Animation = .smooth(duration: 0.35)

    /// `animation`, or nil under Reduce Motion, so the change happens at once.
    /// Every animation in hearth passes through this and nothing is written
    /// inline; callers read `reduceMotion` from the environment.
    /// See docs/design.md#motion
    public static func motion(_ animation: Animation, reduceMotion: Bool) -> Animation? {
        reduceMotion ? nil : animation
    }

    /// The text colour for `tone`; neutral is secondary text.
    public static func color(_ tone: Tone) -> Color {
        switch tone {
        case .neutral: color(.textSecondary)
        case .positive: color(.tonePositive)
        case .attention: color(.toneAttention)
        case .negative: color(.toneNegative)
        }
    }

    /// The background that sits behind `color(tone)`; neutral is the
    /// secondary surface.
    public static func fill(_ tone: Tone) -> Color {
        switch tone {
        case .neutral: color(.surfaceSecondary)
        case .positive: color(.tonePositiveFill)
        case .attention: color(.toneAttentionFill)
        case .negative: color(.toneNegativeFill)
        }
    }

    /// The colour for `role`, resolving to its light or dark value.
    public static func color(_ role: ColorRole) -> Color {
        guard let pair = pair(role) else {
            preconditionFailure("tokens.json has no colour \(role.rawValue)")
        }
        let light = nsColor(hex: pair.light)
        let dark = nsColor(hex: pair.dark)
        return Color(
            nsColor: NSColor(name: nil) { appearance in
                appearance.bestMatch(from: [.aqua, .darkAqua]) == .darkAqua ? dark : light
            }
        )
    }

    static func pair(_ role: ColorRole) -> ColorPair? {
        tokens.colors[role.rawValue]
    }

    static let tokens: Tokens = {
        guard let url = Bundle.module.url(forResource: "tokens", withExtension: "json"),
            let data = try? Data(contentsOf: url),
            let decoded = try? JSONDecoder().decode(Tokens.self, from: data)
        else {
            preconditionFailure("HearthDesign's tokens.json is missing or does not decode")
        }
        return decoded
    }()

    static func nsColor(hex: String) -> NSColor {
        let value = UInt32(hex.dropFirst(), radix: 16) ?? 0
        return NSColor(
            srgbRed: CGFloat((value >> 16) & 0xFF) / 255,
            green: CGFloat((value >> 8) & 0xFF) / 255,
            blue: CGFloat(value & 0xFF) / 255,
            alpha: 1
        )
    }
}

struct ColorPair: Decodable, Sendable {
    let light: String
    let dark: String
}

struct Tokens: Decodable, Sendable {
    let colors: [String: ColorPair]
}
