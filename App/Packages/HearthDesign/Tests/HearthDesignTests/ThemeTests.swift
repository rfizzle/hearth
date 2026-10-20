// Every colour role and tone resolves, and the spacing and radius scales
// climb.

import SwiftUI
import Testing

@testable import HearthDesign

@Test(arguments: ColorRole.allCases)
func everyColorRoleResolves(role: ColorRole) {
    #expect(Theme.pair(role) != nil)
}

@Test(arguments: ColorRole.allCases)
func everyColorRoleHasALightAndADarkHex(role: ColorRole) throws {
    let pair = try #require(Theme.pair(role))
    for hex in [pair.light, pair.dark] {
        #expect(hex.count == 7 && hex.hasPrefix("#"))
        #expect(UInt32(hex.dropFirst(), radix: 16) != nil)
    }
}

@Test func theTokenFileHoldsExactlyTheRoles() {
    #expect(ColorRole.allCases.count == 17)
    #expect(Set(ColorRole.allCases.map(\.rawValue)) == Set(Theme.tokens.colors.keys))
}

@Test(arguments: Tone.allCases)
func everyToneHasAColourAndAFill(tone: Tone) {
    // Resolving is the test: a role with no token traps.
    _ = Theme.color(tone)
    _ = Theme.fill(tone)
}

@Test func spacingAndRadiusAscend() {
    let spacing = [
        Theme.Spacing.xs, Theme.Spacing.sm, Theme.Spacing.md, Theme.Spacing.lg,
        Theme.Spacing.xl, Theme.Spacing.xxl,
    ]
    let radii = [Theme.Radius.sm, Theme.Radius.md, Theme.Radius.lg]
    #expect(spacing == [4, 8, 12, 16, 24, 32])
    #expect(radii == [6, 10, 16])
    #expect(zip(spacing, spacing.dropFirst()).allSatisfy { $0 < $1 })
    #expect(zip(radii, radii.dropFirst()).allSatisfy { $0 < $1 })
}

@Test func motionIsNilUnderReduceMotionAndTheAnimationOtherwise() {
    #expect(Theme.motion(Theme.move, reduceMotion: true) == nil)
    #expect(Theme.motion(Theme.quick, reduceMotion: true) == nil)
    #expect(Theme.motion(Theme.move, reduceMotion: false) == Theme.move)
    #expect(Theme.motion(Theme.quick, reduceMotion: false) == Theme.quick)
}

@Test func chartHuesClampToOneThroughSix() {
    // Resolving is the test: an out-of-range index must not trap.
    for index in [-3, 0, 1, 6, 7, 99] {
        _ = Theme.chart(index)
    }
}
