// The worked line's title and the wording of a burst of tool calls.

import Testing

@testable import HearthUI

@Test func theTitleReadsTheDurationOrNothing() {
    #expect(RunTrace.title(duration: 252) == "Worked for 4m 12s")
    #expect(RunTrace.title(duration: 9) == "Worked for 9s")
    #expect(RunTrace.title(duration: 3780) == "Worked for 1h 3m")
    #expect(RunTrace.title(duration: nil) == "Worked on this")
}

@Test func eachToolTheCoreRegistersHasItsOwnWording() {
    let table: [(String, String)] = [
        ("files.read", "Read 1 file"),
        ("files.write", "Wrote 1 file"),
        ("files.delete", "Deleted 1 file"),
        ("files.render", "Converted 1 file"),
        ("web.fetch", "Read 1 page"),
        ("web.search", "Searched the web"),
        ("deliverables.write", "Made 1 document"),
        ("remember", "Made 1 note"),
        ("ask_owner", "Asked 1 question"),
        ("propose_project", "Proposed 1 project"),
        ("calendar.invite", "Used 1 tool"),
    ]
    for (name, line) in table {
        #expect(ToolBurst.line(names: [name]) == line, "\(name)")
    }
}

@Test func aBurstCountsPerKindInAFixedOrder() {
    let names = ["files.write", "web.search", "files.read", "files.read", "files.read"]
    #expect(ToolBurst.line(names: names) == "Read 3 files, searched the web, wrote 1 file")
    #expect(ToolBurst.line(names: ["web.search", "web.search"]) == "Searched the web 2 times")
    #expect(ToolBurst.line(names: ["x", "files.read", "y"]) == "Read 1 file, used 2 tools")
    #expect(ToolBurst.symbol(names: names) == "doc.text")
    #expect(ToolBurst.symbol(names: ["x"]) == "gearshape")
}
