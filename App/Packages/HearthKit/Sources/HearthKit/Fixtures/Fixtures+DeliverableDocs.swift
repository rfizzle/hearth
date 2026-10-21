// The shaped deliverables the gallery and the tests draw: the five templates
// Hearth ships, copied from the core (a Go test keeps the copies equal), a
// dashboard written in the core's canonical form with every kind of widget and
// three bound to measures, the same file from a newer Hearth, and the values
// a live read of its measures returns, with one gone.

import Foundation

extension Fixtures {
    /// The ids of the five templates Hearth ships, in id order; each is a file
    /// under Resources/Deliverables.
    public static let shippedDeliverableIDs = [
        "comparison", "dashboard", "one-page-brief", "report", "weekly-digest",
    ]

    /// The shipped template `id` as a deliverable, its sample filled in.
    public static func deliverable(_ id: String) -> Deliverable {
        guard
            let url = Bundle.module.url(
                forResource: "\(id).deliverable", withExtension: "json",
                subdirectory: "Deliverables"),
            let data = try? Data(contentsOf: url)
        else {
            preconditionFailure("HearthKit has no shipped deliverable \(id)")
        }
        do {
            return try Deliverable(json: data)
        } catch {
            preconditionFailure("the shipped deliverable \(id) does not decode: \(error)")
        }
    }

    /// The measure ids the kitchen sink binds, as the core resolved them.
    public static let costMeasure = "usage.cost/org/\(measureMonth)"
    public static let tasksMeasure = "project.tasks/project-launch"
    public static let lightMeasure = "project.light/project-launch"

    /// Every kind of widget in one dashboard, as the core writes it: sorted
    /// keys, explicit columns, a bound metric with the time it was read.
    public static let kitchenSinkJSON = """
        {
          "kind": "dashboard",
          "sections": [
            {"columns": 3, "heading": "The numbers", "widgets": [
              {"as_of": "2026-10-20T09:30Z", "caption": "Budget $40", "delta": {"good": true, "value": -3.1},
               "format": "currency", "kind": "metric", "label": "Spent this month",
               "measure": "\(costMeasure)", "value": 12.4},
              {"as_of": "2026-10-20T09:30Z", "caption": "3 of 7 tasks done", "done": 3, "kind": "progress",
               "label": "Launch the API tier", "measure": "\(tasksMeasure)", "total": 7},
              {"as_of": "2026-10-20T09:30Z", "kind": "light", "label": "Launch the API tier",
               "measure": "\(lightMeasure)", "tone": "attention", "word": "Waiting on you"}
            ]},
            {"columns": 2, "heading": "Trends", "widgets": [
              {"categories": ["W40", "W41", "W42", "W43"], "format": "currency", "kind": "line",
               "series": [{"name": "Dana", "values": [3.2, 4.1, 2.8, 5]}, {"name": "Sam", "values": [1, 1.5, 1.2, 2]}],
               "span": 2, "title": "Cost per week", "y_label": "USD"},
              {"categories": ["Done", "Working", "Waiting", "Failed"], "kind": "bar",
               "series": [{"name": "Tasks", "values": [3, 2, 1, 1]}], "title": "Tasks by state"},
              {"format": "currency", "kind": "pie",
               "slices": [{"label": "Dana", "value": 12.4}, {"label": "Sam", "value": 6.1}],
               "title": "Spend by employee"}
            ]},
            {"columns": 1, "heading": "Open items", "widgets": [
              {"columns": [{"header": "Item"}, {"header": "Owner"}, {"align": "right", "header": "Due"}],
               "footer": ["Total", "", 2], "kind": "table",
               "rows": [["Launch email", "Dana", "Fri"], ["Pricing page", "Sam", 3]],
               "title": "Open items"},
              {"items": [{"text": "Send the launch email"},
                         {"detail": "Confirmed Tuesday", "done": true, "text": "Set the price"}],
               "kind": "list", "style": "check", "title": "Next week"}
            ]},
            {"columns": 2, "heading": "Notes", "widgets": [
              {"kind": "callout",
               "markdown": "Two tasks are waiting on an answer. **Open the Inbox** to unblock them.",
               "title": "Needs you", "tone": "attention"},
              {"kind": "text",
               "markdown": "## What happened\\n\\nThe beta invite went out to 240 customers; \
        see [the plan](https://example.com/plan).\\n\\n- one\\n- two\\n\\n\
        | a | b |\\n|---|---|\\n| 1 | 2 |\\n"}
            ]}
          ],
          "subtitle": "Week of 13 October 2026",
          "title": "Office this week",
          "version": 1
        }
        """

    /// The kitchen sink, decoded.
    public static let kitchenSink: Deliverable = {
        do {
            return try Deliverable(json: Data(kitchenSinkJSON.utf8))
        } catch {
            preconditionFailure("the kitchen sink does not decode: \(error)")
        }
    }()

    /// A file a newer Hearth wrote: version 2, which this one does not draw.
    public static let newerDeliverableJSON = """
        {"kind": "dashboard", "sections": [{"widgets": [{"kind": "gauge", "needle": 3}]}],
         "title": "Office next year", "version": 2}
        """

    /// The newer file, decoded: unsupported, with its title.
    public static let newerDeliverable: Deliverable = {
        do {
            return try Deliverable(json: Data(newerDeliverableJSON.utf8))
        } catch {
            preconditionFailure("the newer deliverable does not decode: \(error)")
        }
    }()

    /// What a live read of the kitchen sink's measures returns: the org has
    /// cost $3.45, the launch is 4 of 7 and on track.
    public static let liveMeasures: [String: Hearth_V1_MeasureValue] = [
        costMeasure: .with {
            $0.id = costMeasure
            $0.kind = .number
            $0.label = "Cost · The org · \(measureMonth)"
            $0.unit = "usd_micros"
            $0.number = orgCostMicros
            $0.asOf = time(0)
        },
        tasksMeasure: .with {
            $0.id = tasksMeasure
            $0.kind = .progress
            $0.label = "Tasks done · Launch the API tier"
            $0.unit = "count"
            $0.done = 4
            $0.total = 7
            $0.asOf = time(0)
        },
        lightMeasure: .with {
            $0.id = lightMeasure
            $0.kind = .light
            $0.label = "Project status · Launch the API tier"
            $0.tone = .positive
            $0.word = "On track"
            $0.asOf = time(0)
        },
    ]

    /// The same read once the project is gone: every value missing, with the
    /// core's reason.
    public static let missingMeasures = liveMeasures.mapValues(missingRead)

    private static func missingRead(_ value: Hearth_V1_MeasureValue) -> Hearth_V1_MeasureValue {
        .with {
            $0.id = value.id
            $0.kind = value.kind
            $0.label = value.label
            $0.missing = value.kind == .number ? "Not priced" : "No such project"
        }
    }
}
