// Each primitive declares the states the gallery shows and exposes what it
// wraps: a tone, a pulse, a selection, an expansion or a file type.

import SwiftUI
import Testing
import UniformTypeIdentifiers

@testable import HearthDesign

@Test func statusDotDeclaresItsStates() {
    #expect(
        StatusDot.PreviewState.allCases.map(\.rawValue) == [
            "positive", "attention", "negative", "pulsing",
        ])
    #expect(StatusDot.PreviewState.allCases.filter(\.pulses) == [.pulsing])
    #expect(StatusDot.PreviewState.allCases.allSatisfy { !$0.label.isEmpty })
}

@Test func pillDeclaresAStatePerTone() {
    #expect(Pill.PreviewState.allCases.map(\.tone) == Tone.allCases)
}

@Test func cardSurfaceDeclaresPlainAndSelected() {
    #expect(CardSurface<Text>.PreviewState.allCases.map(\.isSelected) == [false, true])
}

@Test func disclosureRowDeclaresCollapsedAndExpanded() {
    #expect(DisclosureRow<Text, Text>.PreviewState.allCases.map(\.isExpanded) == [false, true])
}

@Test func fileIconDeclaresItsStates() {
    #expect(
        FileIcon.PreviewState.allCases.map(\.rawValue) == [
            "markdown", "pdf", "image", "unknown",
        ])
}

@Test(arguments: [
    ("report.pdf", UTType.pdf), ("chart.png", UTType.png), ("README", UTType.data),
])
func fileIconReadsTheTypeFromTheExtension(filename: String, type: UTType) {
    #expect(FileIcon.contentType(of: filename) == type)
}

@Test func fileIconTakesMarkdownAsText() {
    #expect(FileIcon.contentType(of: "notes.md").conforms(to: .text))
}
