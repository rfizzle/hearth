// The quick ask's shortcut starts as ⌃⌥Space, and listening to it can be
// stopped again, so no test leaves a hot key registered.

import Testing

@testable import HearthNative

@MainActor
@Test func theQuickAskStartsAsControlOptionSpace() {
    let description = GlobalShortcut.quickAsk.defaultDescription ?? ""
    #expect(description.hasPrefix("⌃⌥"))
    #expect(description.lowercased().hasSuffix("space"))
}

@MainActor
@Test func listeningToTheQuickAskCanBeStopped() {
    var pressed = 0
    GlobalShortcut.quickAsk.onPress { pressed += 1 }
    GlobalShortcut.quickAsk.stopListening()
    #expect(pressed == 0)
}
