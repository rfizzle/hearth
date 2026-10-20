// ProjectRow is one project in the sidebar, the way Things lists a project in
// an area: a ProgressPie of done over total tasks as the mark and the name in
// regular weight. An active project with tasks has its light under the name,
// a StatusDot in the tone the core's light names (green positive, yellow
// attention, red negative) with the core's own word beside it, so the colour
// never stands alone; a project with no tasks yet has no light, and a done one
// ends with a check mark in the secondary style. The row is one element to
// VoiceOver: the name, then the progress and the light or "Done" in words.
// The light's word uses the system's .secondary style so sidebar vibrancy
// applies over the glass pane, as a bot's role does.
// See docs/design.md#colour and docs/design.md#window-anatomy

import HearthDesign
import HearthKit
import SwiftUI

/// A project's row in the sidebar.
public struct ProjectRow: View {
    let project: Hearth_V1_Project

    public init(project: Hearth_V1_Project) {
        self.project = project
    }

    public var body: some View {
        HStack(spacing: Theme.Spacing.sm) {
            ProgressPie(
                done: Int(project.status.tasksDone), total: Int(project.status.tasksTotal))
            VStack(alignment: .leading, spacing: 0) {
                Text(project.name).font(Theme.body).lineLimit(1)
                if let tone = Self.tone(of: project) {
                    HStack(spacing: Theme.Spacing.xs) {
                        StatusDot(tone, label: project.status.lightLabel)
                        Text(project.status.lightLabel)
                            .font(Theme.caption)
                            .foregroundStyle(.secondary)
                            .lineLimit(1)
                    }
                }
            }
            Spacer(minLength: Theme.Spacing.sm)
            if project.state == .done {
                Image(systemName: "checkmark.circle")
                    .foregroundStyle(.secondary)
            }
        }
        .accessibilityElement(children: .ignore)
        .accessibilityLabel(project.name)
        .accessibilityValue(Self.spoken(project))
    }

    /// The light's tone, or nil when the row shows no light: no tasks yet, or
    /// the project is not active.
    public static func tone(of project: Hearth_V1_Project) -> Tone? {
        guard project.state == .active else { return nil }
        return switch project.status.light {
        case .green: .positive
        case .yellow: .attention
        case .red: .negative
        default: nil
        }
    }

    /// The row's value as VoiceOver says it.
    public static func spoken(_ project: Hearth_V1_Project) -> String {
        let progress = ProgressPie.spoken(
            done: Int(project.status.tasksDone), total: Int(project.status.tasksTotal))
        if project.state == .done { return "Done" }
        guard tone(of: project) != nil else { return progress }
        return "\(progress), \(project.status.lightLabel)"
    }
}

extension ProjectRow {
    /// The states the gallery and previews show.
    public enum PreviewState: String, CaseIterable, Sendable {
        case green
        case yellow
        case red
        case noTasks = "no tasks"
        case done

        /// The project the row shows in this state.
        public var project: Hearth_V1_Project {
            var project = Fixtures.projects[0]
            switch self {
            case .green:
                project.name = "Refresh the website"
                project.status.light = .green
                project.status.lightLabel = "On track"
                project.status.tasksWaitingOwner = 0
            case .yellow:
                break
            case .red:
                project.name = "Print the catalogue"
                project.status.light = .red
                project.status.lightLabel = "Paused on budget"
                project.status.tasksPausedBudget = 1
            case .noTasks:
                project = Fixtures.projects[1]
            case .done:
                project = Fixtures.projects[2]
            }
            return project
        }
    }

    /// A row showing `state`, for the gallery and previews.
    public init(previewState state: PreviewState) {
        self.init(project: state.project)
    }
}

#Preview("Green") { ProjectRow(previewState: .green).padding() }
#Preview("Yellow") { ProjectRow(previewState: .yellow).padding() }
#Preview("Red") { ProjectRow(previewState: .red).padding() }
#Preview("No tasks") { ProjectRow(previewState: .noTasks).padding() }
#Preview("Done") { ProjectRow(previewState: .done).padding() }
