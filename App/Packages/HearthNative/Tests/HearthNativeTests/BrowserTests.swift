// Browser hands only a page to the system: an absolute .html path with no
// relative parts. Anything else is refused before NSWorkspace is asked.

import Testing

@testable import HearthNative

@Suite("Browser")
struct BrowserTests {
    @Test(
        "an absolute html path is a page",
        arguments: [
            "/Users/owner/Hearth/Dana/Weekly digest.html",
            "/tmp/a.HTML",
        ])
    func page(path: String) {
        #expect(Browser.isPage(path: path))
    }

    @Test(
        "anything else is not",
        arguments: [
            "/Users/owner/Hearth/Dana/digest.pdf",
            "/Users/owner/Hearth/Dana/digest.deliverable.json",
            "/Users/owner/Hearth/../Secrets/a.html",
            "/Users/owner/Hearth/./a.html",
            "Dana/a.html",
            "",
        ])
    func notPage(path: String) {
        #expect(!Browser.isPage(path: path))
    }
}
