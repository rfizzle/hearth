// Prints the CGWindowID of the largest on-screen window the named app owns, so
// screencapture -l can capture just that window. An app also owns a few
// menu-bar-sized windows, hence the largest. The owner name is the bundle's
// display name ("Hearth Gallery", "Hearth"). Owner names, bounds and window
// numbers need no permission; only titles and pixels do.
// Usage: swift scripts/app/windowid.swift [owner name, default Hearth Gallery]

import CoreGraphics
import Foundation

let owner = CommandLine.arguments.dropFirst().first ?? "Hearth Gallery"
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
    print(id)
    exit(0)
}
FileHandle.standardError.write(Data("no window for \(owner)\n".utf8))
exit(1)
