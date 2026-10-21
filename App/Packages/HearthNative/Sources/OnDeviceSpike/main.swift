// Measures the on-device router on ten fixture requests across three teams.
// A separate executable rather than a test: the unit tests must pass on a Mac
// where the model is unavailable, and each call here takes real time.
// Run: swift run --package-path App/Packages/HearthNative OnDeviceSpike

import Foundation
import FoundationModels
import HearthNative

let teams = ["Marketing", "Finance", "Research"]
let fixtures: [(request: String, team: String)] = [
    ("Write the launch email for the API tier", "Marketing"),
    ("Plan a six-week campaign for small agencies", "Marketing"),
    ("Which channels should we use for the spring launch?", "Marketing"),
    ("How much did we spend on software last month?", "Finance"),
    ("Set up a budget for the new year", "Finance"),
    ("How many months of runway do we have?", "Finance"),
    ("What do our competitors charge for API access?", "Research"),
    ("Compare three CRM tools for a ten-person team", "Research"),
    ("How big is the market for AI tools in law firms?", "Research"),
    ("Draft the monthly finance report and tell me what moved", "Finance"),
]

print("availability: \(SystemLanguageModel.default.availability)")
guard OnDeviceAvailability.system.isAvailable() else {
    print("not runnable on this Mac: the model is not available")
    exit(0)
}

let router = OnDeviceRouter()
var hits = 0
var total = Duration.zero
for fixture in fixtures {
    let clock = ContinuousClock()
    var hint: RouteHint?
    let took = await clock.measure {
        hint = await router.route(request: fixture.request, teams: teams)
    }
    total += took
    let correct = hint?.team == fixture.team
    if correct { hits += 1 }
    print(
        "\(correct ? "hit " : "miss") \(took) want=\(fixture.team) got=\(hint?.team ?? "nil") :: \(fixture.request)"
    )
}
print("hits: \(hits)/\(fixtures.count), mean latency: \(total / fixtures.count)")

let long = String(repeating: "The quick brown fox jumps over the lazy dog. ", count: 400)
let title = await OnDeviceTitler().title(for: long)
print("title for \(long.count) characters: \(title ?? "nil")")
