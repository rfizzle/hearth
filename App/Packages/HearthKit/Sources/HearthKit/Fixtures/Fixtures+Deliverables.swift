// The office's figures and the deliverable templates the preview serves: the
// launch project at 3 of 7 with its light, what the org has cost this
// month, the Inbox count, and a project that is gone.

import Foundation

extension Fixtures {
    /// The month the usage figures are for.
    public static let measureMonth = "2026-10"

    /// An id no project has, which reads as missing.
    public static let goneProjectMeasure = "project.tasks/project-gone"

    /// What the org has cost in `measureMonth`, in micro-dollars.
    public static let orgCostMicros = 3_450_000.0

    /// Open items in the Inbox, as the preview's Inbox count reads.
    public static let inboxOpen = 2.0

    /// A scripted subscription: the launch project's fourth task is done,
    /// so its status arrives with 4 of 7. A client that is handed it serves
    /// 4 of 7 on the next read of `project.tasks/project-launch`.
    public static let measureEvents: [Hearth_V1_SubscribeResponse] = {
        var changed = projects[0]
        changed.status.tasksDone += 1
        changed.status.tasksWorking -= 1
        changed.updatedAt = time(40)
        return [
            .with {
                $0.sequence = 1
                $0.event = .projectChanged(.with { $0.project = changed })
            }
        ]
    }()

    /// The five templates Hearth ships, in id order.
    public static let shippedTemplates: [Hearth_V1_DeliverableTemplate] = [
        ("comparison", "Comparison", "Two or more options set side by side, with a verdict."),
        ("dashboard", "Dashboard", "The office's own numbers, kept live."),
        (
            "one-page-brief", "One-page brief",
            "The ask, the context, the options and a recommendation."
        ),
        ("report", "Report", "A finding and a recommendation."),
        (
            "weekly-digest", "Weekly digest",
            "A short recap of a week: what happened, the numbers that moved, what is next."
        ),
    ].map { id, name, when in
        .with {
            $0.id = id
            $0.name = name
            $0.when = when
            $0.shipped = true
        }
    }
}
