// PreviewClient's measure and template calls and the list the templates
// change, in a file of their own to keep both short. A measure reads the
// projects the client keeps, so an event that changes a project (handed to
// `deliver`) changes the next read.

import Foundation

extension PreviewClient {
    public func listMeasures() async throws -> [Hearth_V1_MeasureValue] {
        var ids = projectList.all().filter { $0.state != .archived }.flatMap {
            ["project.tasks/\($0.id)", "project.light/\($0.id)"]
        }
        ids += ["usage.cost/org/\(Fixtures.measureMonth)", "inbox.open"]
        return ids.map { id in
            var value = measure(id)
            value = .with {
                $0.id = value.id
                $0.kind = value.kind
                $0.label = value.label
                $0.unit = value.unit
            }
            return value
        }
    }

    public func readMeasures(ids: [String]) async throws -> [Hearth_V1_MeasureValue] {
        guard ids.count <= 100 else {
            throw PageRefusal(message: "at most 100 ids at a time; \(ids.count) were asked")
        }
        measureReads.record(ids)
        return ids.map(measure)
    }

    /// Hands the client an event the core would have sent, so the figures it
    /// serves move with it: a changed project changes its measures.
    public func deliver(_ event: Hearth_V1_SubscribeResponse) {
        if case .projectChanged(let changed) = event.event {
            projectList.replace(changed.project)
        }
    }

    public func listDeliverableTemplates() async throws -> [Hearth_V1_DeliverableTemplate] {
        Fixtures.shippedTemplates + templateList.all()
    }

    public func saveDeliverableTemplate(
        json: String
    ) async throws -> Hearth_V1_DeliverableTemplate {
        try templateList.save(json)
    }

    public func deleteDeliverableTemplate(id: String) async throws {
        try templateList.delete(id)
    }

    private func measure(_ id: String) -> Hearth_V1_MeasureValue {
        let now = Fixtures.time(0)
        var value = Hearth_V1_MeasureValue.with {
            $0.id = id
            $0.asOf = now
        }
        let parts = id.split(separator: "/").map(String.init)
        switch parts.first {
        case "project.tasks", "project.light":
            guard parts.count == 2,
                let project = projectList.all().first(where: { $0.id == parts[1] })
            else {
                value.kind = parts.first == "project.light" ? .light : .progress
                value.label = "Project"
                value.missing = "No such project"
                return value
            }
            if parts[0] == "project.tasks" {
                value.kind = .progress
                value.label = "Tasks done · " + project.name
                value.unit = "count"
                value.done = Int64(project.status.tasksDone)
                value.total = Int64(project.status.tasksTotal)
            } else {
                value.kind = .light
                value.label = "Project status · " + project.name
                value.word = project.status.lightLabel
                value.tone = Self.tone(project.status.light)
            }
        case "usage.cost":
            value.kind = .number
            value.label = "Cost · The org · " + Fixtures.measureMonth
            value.unit = "usd_micros"
            value.number = Fixtures.orgCostMicros
        case "inbox.open":
            value.kind = .number
            value.label = "Open in the Inbox"
            value.unit = "count"
            value.number = Fixtures.inboxOpen
        default:
            value.missing = "Not a measure the preview serves"
        }
        return value
    }

    private static func tone(_ light: Hearth_V1_ProjectLight) -> Hearth_V1_Tone {
        switch light {
        case .green: .positive
        case .yellow: .attention
        case .red: .negative
        default: .neutral
        }
    }
}

/// The reads of measures a PreviewClient has served.
public final class PreviewMeasureReads: @unchecked Sendable {
    private let lock = NSLock()
    private var reads: [[String]] = []

    /// The ids of each read, in order.
    public var all: [[String]] { lock.withLock { reads } }

    func record(_ ids: [String]) { lock.withLock { reads.append(ids) } }
}

/// The owner's templates PreviewClient serves, kept so a save or a delete
/// shows in the next list.
final class TemplateList: @unchecked Sendable {
    private let lock = NSLock()
    private var templates: [Hearth_V1_DeliverableTemplate] = []

    func all() -> [Hearth_V1_DeliverableTemplate] {
        lock.withLock { templates }
    }

    /// Keeps the document under the id in its `template` header, or refuses it
    /// in words as the core does.
    func save(_ json: String) throws -> Hearth_V1_DeliverableTemplate {
        guard
            let top = try? JSONSerialization.jsonObject(with: Data(json.utf8)) as? [String: Any],
            let head = top["template"] as? [String: Any],
            let id = head["id"] as? String, !id.isEmpty
        else {
            throw PageRefusal(message: "the template is not a template document")
        }
        if Fixtures.shippedTemplates.contains(where: { $0.id == id }) {
            throw PageRefusal(message: "\(id) is a shipped template; pick another id")
        }
        let saved = Hearth_V1_DeliverableTemplate.with {
            $0.id = id
            $0.name = head["name"] as? String ?? id
            $0.when = head["when"] as? String ?? ""
            $0.json = json
            $0.path = Fixtures.workspacePath + "/Templates/" + id + ".deliverable.json"
        }
        return lock.withLock {
            templates.removeAll { $0.id == id }
            templates.append(saved)
            templates.sort { $0.id < $1.id }
            return saved
        }
    }

    func delete(_ id: String) throws {
        try lock.withLock {
            guard templates.contains(where: { $0.id == id }) else {
                throw PageRefusal(message: "there is no template \(id)")
            }
            templates.removeAll { $0.id == id }
        }
    }
}
