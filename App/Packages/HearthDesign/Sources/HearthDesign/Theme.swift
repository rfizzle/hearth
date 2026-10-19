// Theme turns the tokens in Resources/tokens.json into SwiftUI values by role.
// A colour token carries a light and a dark value; the NSColor behind it
// resolves against the current appearance, so views never branch on it.
// make tokens-check reads the same file and checks every pairing's contrast.

import AppKit
import SwiftUI

/// A colour named by what it is for, not what it looks like.
public enum ColorRole: String, CaseIterable, Sendable {
    case textPrimary = "text.primary"
    case textSecondary = "text.secondary"
    case surfacePrimary = "surface.primary"
}

/// Tokens by role.
public enum Theme {
    /// Body text.
    public static let body: Font = .body

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
