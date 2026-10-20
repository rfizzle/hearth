// ProjectForm is the project sheet's draft: the name, the brief's four parts
// and an optional target date, as typed. It starts blank for a new project or
// from a project being edited, and turns into the create or edit request. The
// one check it makes is the sheet's: Save is on once the name and every part
// of the brief are typed; whether they are any good is the core's call. A
// target date travels as yyyy-mm-dd, the calendar day the picker shows.
// See docs/model.md#work

import Foundation
import HearthKit

/// The fields of the new-project and edit-project sheet.
public struct ProjectForm: Equatable, Sendable {
    public var name = ""
    public var goal = ""
    public var context = ""
    public var doneLooksLike = ""
    public var doneWhen = ""
    public var hasTargetDate = false
    public var targetDate = Date()

    /// A blank form for a new project.
    public init() {}

    /// A form holding `project`'s name, brief and target date, for editing.
    public init(project: Hearth_V1_Project) {
        name = project.name
        goal = project.brief.goal
        context = project.brief.context
        doneLooksLike = project.brief.doneLooksLike
        doneWhen = project.brief.doneWhen
        if let date = Self.date(from: project.targetDate) {
            hasTargetDate = true
            targetDate = date
        }
    }

    /// Whether Save is on: the name and every part of the brief are typed.
    public var canSave: Bool {
        [name, goal, context, doneLooksLike, doneWhen].allSatisfy {
            !$0.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
        }
    }

    /// The brief as the contract carries it.
    var brief: Hearth_V1_ProjectBrief {
        .with {
            $0.goal = goal
            $0.context = context
            $0.doneLooksLike = doneLooksLike
            $0.doneWhen = doneWhen
        }
    }

    /// The target date as yyyy-mm-dd, or empty for none.
    var targetDateText: String {
        hasTargetDate ? Self.text(from: targetDate) : ""
    }

    /// The request that creates this project.
    public var createRequest: Hearth_V1_CreateProjectRequest {
        .with {
            $0.name = name
            $0.brief = brief
            $0.targetDate = targetDateText
        }
    }

    /// The request that replaces the project with `id`'s name, brief and date.
    public func updateRequest(id: String) -> Hearth_V1_UpdateProjectRequest {
        .with {
            $0.id = id
            $0.name = name
            $0.brief = brief
            $0.targetDate = targetDateText
        }
    }

    private static func formatter() -> DateFormatter {
        let formatter = DateFormatter()
        formatter.calendar = Calendar(identifier: .gregorian)
        formatter.locale = Locale(identifier: "en_US_POSIX")
        formatter.timeZone = .current
        formatter.dateFormat = "yyyy-MM-dd"
        return formatter
    }

    static func date(from text: String) -> Date? {
        text.isEmpty ? nil : formatter().date(from: text)
    }

    static func text(from date: Date) -> String {
        formatter().string(from: date)
    }
}
