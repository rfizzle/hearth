// ProjectHeader is a project's brief, pinned above its conversation so what it
// is for is never out of sight. It sits flat on the content layer, no card:
// the "done when" sentence in the headline type as the project's one-line
// identity, then a row with the ProgressPie and its words, the core's light as
// a StatusDot with its own word, and the target date, which turns to "Was due"
// in the attention tone once it has passed on a project not done. A done
// project shows "Done 3 Nov" in the positive tone where the light was. When
// every task is done an inline prominent Mark as Done asks for the owner's
// word. A "Brief" disclosure row opens to the goal, the context and what done
// looks like. The header is one group to VoiceOver, named by the sentence, and
// the button and the row inside it keep their own roles. The status, the
// light and the words are the core's; this only lays them out.
// See docs/design.md#colour and docs/model.md#work

import HearthDesign
import HearthKit
import SwiftUI

/// A project's brief and progress.
public struct ProjectHeader: View {
    let project: Hearth_V1_Project
    let onMarkDone: () -> Void
    let today: Date
    @State private var briefOpen: Bool

    /// A header for `project`; `briefOpen` starts the Brief row open, and
    /// `today` is the day the target date is judged against.
    public init(
        project: Hearth_V1_Project, briefOpen: Bool = false, today: Date = Date(),
        onMarkDone: @escaping () -> Void = {}
    ) {
        self.project = project
        self.today = today
        self.onMarkDone = onMarkDone
        _briefOpen = State(initialValue: briefOpen)
    }

    public var body: some View {
        VStack(alignment: .leading, spacing: Theme.Spacing.xs) {
            Text(project.brief.doneWhen)
                .font(Theme.headline)
                .frame(maxWidth: .infinity, alignment: .leading)
            statusRow
            DisclosureRow(
                isExpanded: $briefOpen,
                label: { Text("Brief") },
                content: {
                    VStack(alignment: .leading, spacing: Theme.Spacing.sm) {
                        part("Goal", project.brief.goal)
                        part("Context", project.brief.context)
                        part("What done looks like", project.brief.doneLooksLike)
                    }
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .padding(.bottom, Theme.Spacing.sm)
                })
        }
        .accessibilityElement(children: .contain)
        .accessibilityLabel(project.brief.doneWhen)
    }

    private var statusRow: some View {
        HStack(spacing: Theme.Spacing.md) {
            HStack(spacing: Theme.Spacing.sm) {
                ProgressPie(
                    done: Int(project.status.tasksDone), total: Int(project.status.tasksTotal))
                Text(Self.progress(project)).font(Theme.callout)
            }
            .accessibilityElement(children: .combine)
            if let tone = ProjectRow.tone(of: project) {
                HStack(spacing: Theme.Spacing.xs) {
                    StatusDot(tone, label: project.status.lightLabel)
                    Text(project.status.lightLabel)
                        .font(Theme.caption)
                        .foregroundStyle(.secondary)
                }
                .accessibilityElement(children: .combine)
            }
            if let line = Self.dateLine(of: project, today: today) {
                Text(line.text)
                    .font(Theme.caption)
                    .foregroundStyle(
                        line.tone == .neutral
                            ? AnyShapeStyle(.secondary) : AnyShapeStyle(Theme.color(line.tone)))
            }
            Spacer(minLength: 0)
            if project.state == .active, project.status.allTasksDone {
                Button("Mark as Done", action: onMarkDone)
                    .buttonStyle(.borderedProminent)
                    .controlSize(.small)
            }
        }
    }

    private func part(_ title: String, _ text: String) -> some View {
        VStack(alignment: .leading, spacing: 0) {
            Text(title)
                .font(Theme.caption)
                .foregroundStyle(.secondary)
            Text(text)
                .font(Theme.body)
                .textSelection(.enabled)
        }
    }

    /// The progress in words: "3 of 7 done", "All done" or "No tasks yet".
    public static func progress(_ project: Hearth_V1_Project) -> String {
        ProgressPie.spoken(
            done: Int(project.status.tasksDone), total: Int(project.status.tasksTotal))
    }

    /// The date line: "Done 3 Nov" in the positive tone for a done project,
    /// "Was due 14 Nov" in the attention tone for an active one past its
    /// target, "Due 14 Nov" otherwise, or nil with no date to show.
    public static func dateLine(
        of project: Hearth_V1_Project, today: Date, timeZone: TimeZone = .current
    ) -> (text: String, tone: Tone)? {
        if project.state == .done {
            guard project.hasDoneAt else { return nil }
            return ("Done \(shortDay(project.doneAt.date, timeZone: timeZone))", .positive)
        }
        guard !project.targetDate.isEmpty,
            let due = day(from: project.targetDate, timeZone: timeZone)
        else { return nil }
        var calendar = Calendar(identifier: .gregorian)
        calendar.timeZone = timeZone
        let late = calendar.startOfDay(for: today) > due
        let word = shortDay(due, timeZone: timeZone)
        return late ? ("Was due \(word)", .attention) : ("Due \(word)", .neutral)
    }

    private static func formatter(_ format: String, _ timeZone: TimeZone) -> DateFormatter {
        let formatter = DateFormatter()
        formatter.calendar = Calendar(identifier: .gregorian)
        formatter.locale = Locale(identifier: "en_US_POSIX")
        formatter.timeZone = timeZone
        formatter.dateFormat = format
        return formatter
    }

    private static func day(from text: String, timeZone: TimeZone) -> Date? {
        formatter("yyyy-MM-dd", timeZone).date(from: text)
    }

    private static func shortDay(_ date: Date, timeZone: TimeZone) -> String {
        formatter("d MMM", timeZone).string(from: date)
    }
}

extension ProjectHeader {
    /// The states the gallery and previews show.
    public enum PreviewState: String, CaseIterable, Sendable {
        case green
        case yellow
        case red
        case overdue
        case allDone = "all tasks done"
        case noTasks = "no tasks"
        case done
        case briefOpen = "brief open"

        /// The project the header shows in this state.
        public var project: Hearth_V1_Project {
            var project = Fixtures.projects[0]
            switch self {
            case .green:
                project.status.light = .green
                project.status.lightLabel = "On track"
                project.status.tasksWaitingOwner = 0
            case .yellow, .briefOpen:
                break
            case .red:
                project.status.light = .red
                project.status.lightLabel = "Paused on budget"
                project.status.tasksPausedBudget = 1
            case .overdue:
                project.targetDate = "2026-09-30"
            case .allDone:
                project.status.tasksDone = 7
                project.status.tasksWorking = 0
                project.status.tasksWaitingOwner = 0
                project.status.tasksCancelled = 0
                project.status.light = .green
                project.status.lightLabel = "All tasks done"
                project.status.allTasksDone = true
            case .noTasks:
                project = Fixtures.projects[1]
            case .done:
                project = Fixtures.projects[2]
            }
            return project
        }
    }

    /// A header showing `state`, for the gallery and previews. Today is fixed
    /// so the date line reads the same each time.
    public init(previewState state: PreviewState) {
        self.init(
            project: state.project, briefOpen: state == .briefOpen,
            today: Date(timeIntervalSince1970: 1_792_486_800))
    }
}

#Preview("Yellow") { ProjectHeader(previewState: .yellow).padding() }
#Preview("All tasks done") { ProjectHeader(previewState: .allDone).padding() }
#Preview("Done") { ProjectHeader(previewState: .done).padding() }
