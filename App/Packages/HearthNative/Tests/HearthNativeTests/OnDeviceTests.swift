import Testing

@testable import HearthNative

@Test func routerIsNilWhenTheModelIsUnavailable() async {
    let router = OnDeviceRouter(availability: .fixed(false))
    let hint = await router.route(
        request: "Plan the API tier launch", teams: ["Marketing", "Finance"])
    #expect(hint == nil)
}

@Test func routerIsNilWithoutTeamsOrRequest() async {
    let router = OnDeviceRouter(availability: .fixed(true))
    #expect(await router.route(request: "hello", teams: []) == nil)
    #expect(await router.route(request: "  \n", teams: ["Marketing"]) == nil)
}

@Test func titlerIsNilWhenTheModelIsUnavailable() async {
    let titler = OnDeviceTitler(availability: .fixed(false))
    #expect(await titler.title(for: "Notes on the launch meeting") == nil)
}

@Test func titlerCutsLongInput() {
    let long = String(repeating: "a", count: OnDeviceTitler.maxInputCharacters * 2)
    #expect(OnDeviceTitler.truncated(long).count == OnDeviceTitler.maxInputCharacters)
    #expect(OnDeviceTitler.truncated("  short \n") == "short")
}

@Test(arguments: [
    ("\"Launch meeting notes.\"", "Launch meeting notes"),
    ("  Three  word title ", "Three word title"),
    ("one two three four five six seven", nil),
    ("   ", nil),
])
func titleCleaning(raw: String, want: String?) {
    #expect(OnDeviceTitler.clean(raw) == want)
}
