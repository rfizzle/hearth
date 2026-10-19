// Placeholder test for HearthDesign: every colour role has a token.

import Testing

@testable import HearthDesign

@Test(arguments: ColorRole.allCases)
func everyColorRoleResolves(role: ColorRole) {
    #expect(Theme.pair(role) != nil)
}
