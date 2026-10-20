// Prints the CGWindowID of the largest on-screen window the named app owns, so
// screencapture -l can capture just that window. An app also owns a few
// menu-bar-sized windows, hence the largest. The owner name is the bundle's
// display name ("Hearth Gallery", "Hearth"). Owner names, bounds and window
// numbers need no permission; only titles and pixels do.
// With --bounds it prints the window's rectangle as x,y,w,h in points instead,
// for screencapture -R, which records a region (-l is stills only).
// Usage: swift scripts/app/windowid.swift [--bounds] [owner name, default Hearth Gallery]

import CoreGraphics
import Foundation

let wantsBounds = CommandLine.arguments.contains("--bounds")
let owner = CommandLine.arguments.dropFirst().first { $0 != "--bounds" } ?? "Hearth Gallery"
let windows =
    CGWindowListCopyWindowInfo([.optionOnScreenOnly, .excludeDesktopElements], kCGNullWindowID)
    as? [[String: Any]] ?? []

func area(_ window: [String: Any]) -> Double {
    guard let bounds = window[kCGWindowBounds as String] as? [String: Double] else { return 0 }
    return (bounds["Width"] ?? 0) * (bounds["Height"] ?? 0)
}

let ours = windows.filter {
    $0[kCGWindowOwnerName as String] as? String == owner
        && $0[kCGWindowLayer as String] as? Int == 0
}
if let window = ours.max(by: { area($0) < area($1) }),
    let id = window[kCGWindowNumber as String] as? Int
{
    if wantsBounds, let bounds = window[kCGWindowBounds as String] as? [String: Double] {
        let parts = ["X", "Y", "Width", "Height"].map { Int((bounds[$0] ?? 0).rounded()) }
        print(parts.map(String.init).joined(separator: ","))
    } else {
        print(id)
    }
    exit(0)
}
FileHandle.standardError.write(Data("no window for \(owner)\n".utf8))
exit(1)
