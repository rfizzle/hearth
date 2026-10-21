// The states WidgetControl shows in previews and the gallery: each of the
// seven kinds empty, valid, invalid (over a limit, out of range, or a
// required control left empty where nothing else can be wrong) and answered,
// over one sample question per kind. The prompt card's previews reuse the
// same samples.

import Foundation
import HearthKit
import SwiftUI

/// One sample question per kind, and the owner's answers to it.
enum WidgetSample {
    /// The seven kinds, in the catalogue's order.
    enum Kind: CaseIterable {
        case single, multiple, freeText, yesNo, date, path, scale
    }

    /// Where a control is in its preview.
    enum Phase: CaseIterable {
        case empty, valid, invalid, answered
    }

    /// The sample question for `kind`.
    static func widget(_ kind: Kind) -> Hearth_V1_Widget {
        switch kind {
        case .single: single
        case .multiple: multiple
        case .freeText:
            .with {
                $0.freeText = .with {
                    $0.placeholder = "Plans, prices, the customers you lose to them…"
                    $0.maxLength = 120
                    $0.multiline = true
                }
            }
        case .yesNo:
            .with {
                $0.required = true
                $0.yesNo = .with {
                    $0.yesLabel = "Keep it"
                    $0.noLabel = "Delete it"
                }
            }
        case .date:
            .with {
                $0.required = true
                $0.date = .with {
                    $0.earliest = .init(date: utcDay(2026, 10, 21))
                    $0.latest = .init(date: utcDay(2026, 11, 30))
                }
            }
        case .path: path
        case .scale: scale
        }
    }

    private static let single = Hearth_V1_Widget.with {
        $0.required = true
        $0.singleChoice = .with {
            $0.options = [
                option("one-page", "One page", "The short brief you asked for"),
                option("two-pages", "Two pages", "Room for a price table"),
                option("slides", "Five slides"),
            ]
            $0.allowOther = true
        }
    }

    private static let multiple = Hearth_V1_Widget.with {
        $0.multipleChoice = .with {
            $0.options = [
                option("northwind", "Northwind"), option("quillmate", "Quillmate"),
                option("draftly", "Draftly"), option("penwise", "Penwise"),
                option("scribeworks", "Scribeworks", "Sells only to large firms"),
            ]
            $0.max = 3
        }
    }

    private static let path = Hearth_V1_Widget.with {
        $0.path = .with {
            $0.folders = true
            $0.files = true
            $0.startPath = Fixtures.bot.folderPath
        }
    }

    private static let scale = Hearth_V1_Widget.with {
        $0.required = true
        $0.scale = .with {
            $0.min = 1
            $0.max = 5
            $0.minLabel = "Not useful"
            $0.maxLabel = "Very useful"
        }
    }

    /// The owner's answer to `kind`'s sample in `phase`.
    static func draft(_ kind: Kind, _ phase: Phase) -> WidgetDraft {
        var draft = WidgetDraft()
        switch phase {
        case .empty: break
        case .valid, .answered: fillValid(&draft, kind)
        case .invalid: fillInvalid(&draft, kind)
        }
        return draft
    }

    private static func fillValid(_ draft: inout WidgetDraft, _ kind: Kind) {
        switch kind {
        case .single: draft.choice = "two-pages"
        case .multiple: draft.choices = ["quillmate", "scribeworks"]
        case .freeText: draft.text = "Starter is $20 a month, and most customers ask about the API."
        case .yesNo: draft.yes = true
        case .date: draft.date = localDay(2026, 10, 24)
        case .path: draft.path = Fixtures.bot.folderPath + "/Competitor pricing brief.md"
        case .scale: draft.scale = 4
        }
    }

    private static func fillInvalid(_ draft: inout WidgetDraft, _ kind: Kind) {
        switch kind {
        case .single: draft.choosesOther = true
        case .multiple: draft.choices = ["northwind", "quillmate", "draftly", "scribeworks"]
        case .freeText:
            draft.text = String(
                repeating: "Starter is $20 a month, and most customers ask about the API. ",
                count: 3)
        case .date: draft.date = localDay(2026, 12, 25)
        // Nothing can be out of bounds here: a required control left empty.
        case .yesNo, .path, .scale: break
        }
    }

    static func option(_ id: String, _ label: String, _ note: String = "") -> Hearth_V1_Choice {
        .with {
            $0.id = id
            $0.label = label
            $0.detail = note
        }
    }

    private static func utcDay(_ year: Int, _ month: Int, _ day: Int) -> Date {
        var calendar = Calendar(identifier: .gregorian)
        calendar.timeZone = .gmt
        return calendar.date(from: DateComponents(year: year, month: month, day: day)) ?? .now
    }

    private static func localDay(_ year: Int, _ month: Int, _ day: Int) -> Date {
        // The samples are written on the Gregorian calendar, in this Mac's zone.
        Calendar(identifier: .gregorian).date(
            from: DateComponents(timeZone: .current, year: year, month: month, day: day)) ?? .now
    }
}

extension WidgetControl {
    /// The states the gallery and previews show, kind by kind.
    public enum PreviewState: String, CaseIterable, Sendable {
        case singleEmpty, singleValid, singleInvalid, singleAnswered
        case multipleEmpty, multipleValid, multipleInvalid, multipleAnswered
        case freeTextEmpty, freeTextValid, freeTextInvalid, freeTextAnswered
        case yesNoEmpty, yesNoValid, yesNoInvalid, yesNoAnswered
        case dateEmpty, dateValid, dateInvalid, dateAnswered
        case pathEmpty, pathValid, pathInvalid, pathAnswered
        case scaleEmpty, scaleValid, scaleInvalid, scaleAnswered

        // Declared kind by kind, four phases each.
        private var index: Int { Self.allCases.firstIndex(of: self) ?? 0 }

        var kind: WidgetSample.Kind {
            WidgetSample.Kind.allCases[index / WidgetSample.Phase.allCases.count]
        }

        var phase: WidgetSample.Phase {
            WidgetSample.Phase.allCases[index % WidgetSample.Phase.allCases.count]
        }

        var widget: Hearth_V1_Widget { WidgetSample.widget(kind) }

        var draft: WidgetDraft { WidgetSample.draft(kind, phase) }
    }

    /// A control showing `state`, for the gallery and previews; an answered
    /// one cannot be changed.
    public init(previewState state: PreviewState) {
        self.init(widget: state.widget, draft: .constant(state.draft))
        isLocked = state.phase == .answered
    }
}

#Preview("Single, empty") { WidgetControl(previewState: .singleEmpty).frame(width: 480).padding() }
#Preview("Single, valid") { WidgetControl(previewState: .singleValid).frame(width: 480).padding() }
#Preview("Single, invalid") {
    WidgetControl(previewState: .singleInvalid).frame(width: 480).padding()
}
#Preview("Single, answered") {
    WidgetControl(previewState: .singleAnswered).frame(width: 480).padding()
}
#Preview("Multiple, empty") {
    WidgetControl(previewState: .multipleEmpty).frame(width: 480).padding()
}
#Preview("Multiple, valid") {
    WidgetControl(previewState: .multipleValid).frame(width: 480).padding()
}
#Preview("Multiple, invalid") {
    WidgetControl(previewState: .multipleInvalid).frame(width: 480).padding()
}
#Preview("Multiple, answered") {
    WidgetControl(previewState: .multipleAnswered).frame(width: 480).padding()
}
#Preview("Free text, empty") {
    WidgetControl(previewState: .freeTextEmpty).frame(width: 480).padding()
}
#Preview("Free text, valid") {
    WidgetControl(previewState: .freeTextValid).frame(width: 480).padding()
}
#Preview("Free text, invalid") {
    WidgetControl(previewState: .freeTextInvalid).frame(width: 480).padding()
}
#Preview("Free text, answered") {
    WidgetControl(previewState: .freeTextAnswered).frame(width: 480).padding()
}
#Preview("Yes or no, empty") {
    WidgetControl(previewState: .yesNoEmpty).frame(width: 480).padding()
}
#Preview("Yes or no, valid") {
    WidgetControl(previewState: .yesNoValid).frame(width: 480).padding()
}
#Preview("Yes or no, invalid") {
    WidgetControl(previewState: .yesNoInvalid).frame(width: 480).padding()
}
#Preview("Yes or no, answered") {
    WidgetControl(previewState: .yesNoAnswered).frame(width: 480).padding()
}
#Preview("Date, empty") { WidgetControl(previewState: .dateEmpty).frame(width: 480).padding() }
#Preview("Date, valid") { WidgetControl(previewState: .dateValid).frame(width: 480).padding() }
#Preview("Date, invalid") { WidgetControl(previewState: .dateInvalid).frame(width: 480).padding() }
#Preview("Date, answered") {
    WidgetControl(previewState: .dateAnswered).frame(width: 480).padding()
}
#Preview("Path, empty") { WidgetControl(previewState: .pathEmpty).frame(width: 480).padding() }
#Preview("Path, valid") { WidgetControl(previewState: .pathValid).frame(width: 480).padding() }
#Preview("Path, invalid") { WidgetControl(previewState: .pathInvalid).frame(width: 480).padding() }
#Preview("Path, answered") {
    WidgetControl(previewState: .pathAnswered).frame(width: 480).padding()
}
#Preview("Scale, empty") { WidgetControl(previewState: .scaleEmpty).frame(width: 480).padding() }
#Preview("Scale, valid") { WidgetControl(previewState: .scaleValid).frame(width: 480).padding() }
#Preview("Scale, invalid") {
    WidgetControl(previewState: .scaleInvalid).frame(width: 480).padding()
}
#Preview("Scale, answered") {
    WidgetControl(previewState: .scaleAnswered).frame(width: 480).padding()
}
