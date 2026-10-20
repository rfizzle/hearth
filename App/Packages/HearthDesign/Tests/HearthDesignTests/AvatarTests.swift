// Avatar takes initials from the first two words of a name and declares the
// states the gallery shows.

import Testing

@testable import HearthDesign

@MainActor
@Test(arguments: [
    ("Dana Reyes", "DR"), ("dana", "D"), ("Mary Ann Lee", "MA"), ("", ""),
])
func initialsComeFromTheFirstTwoWords(name: String, initials: String) {
    #expect(Avatar.initials(of: name) == initials)
}

@MainActor
@Test func avatarHasAStatePerContent() {
    #expect(
        Avatar.PreviewState.allCases.map(\.content) == [
            .initials("Dana Reyes"), .symbol("magnifyingglass"), .character("🦊"),
            .initials("Dana Reyes"),
        ])
}

@MainActor
@Test func theLargeStateIsTheOnlyLargeOne() {
    #expect(Avatar.PreviewState.allCases.filter { $0.size == .large } == [.large])
    #expect(Avatar.Size.regular.diameter == Theme.Size.avatar)
    #expect(Avatar.Size.large.diameter == Theme.Size.avatarLarge)
}
