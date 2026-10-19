// Measures the on-device router on ten fixture requests across three teams.
// A separate executable rather than a test: the unit tests must pass on a Mac
// where the model is unavailable, and each call here takes real time.
// Run: swift run --package-path App/Packages/HearthNative OnDeviceSpike

import Foundation
import FoundationModels
import HearthNative

let teams = ["Travel", "Money", "Home"]
let fixtures: [(request: String, team: String)] = [
    ("Find me a flight to Lisbon in March", "Travel"),
    ("Book a hotel near the conference in Austin", "Travel"),
    ("What should I pack for ten days in Japan?", "Travel"),
    ("How much did I spend on restaurants last month?", "Money"),
    ("Set up a budget for the new year", "Money"),
    ("Is my quarterly tax estimate due soon?", "Money"),
    ("The kitchen tap is dripping, who should I call?", "Home"),
    ("Make a cleaning schedule for the week", "Home"),
    ("Remind me when the boiler service is due", "Home"),
    ("Plan a weekend trip to Vermont and tell me the cost", "Travel"),
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
