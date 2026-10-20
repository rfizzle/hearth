// SidebarStore is what the sidebar has selected, over the two lists it shows:
// the Inbox, a bot or a project, one selection for the whole sidebar, which
// the detail column opens. It decides nothing: a selected bot or project that
// has left its list resolves to nothing, and with nothing selected yet the
// Chief of Staff, first in the Office, is. The menu bar's rows select what
// holds a conversation: its bot, its project, or the Inbox.
// See docs/design.md#window-anatomy

import HearthKit
import Observation

/// What the sidebar can select.
public enum SidebarSelection: Hashable, Sendable {
    /// What waits on the owner.
    case inbox
    /// The bot with this id.
    case bot(String)
    /// The project with this id.
    case project(String)
}

/// The sidebar's selection over the bots and the projects.
@MainActor
@Observable
public final class SidebarStore {
    /// The bots: the Office and the team.
    public let bots: BotListStore
    /// The projects.
    public let projects: ProjectsStore
    /// What the sidebar has selected, if anything.
    public var selected: SidebarSelection?

    /// A sidebar over `bots` and `projects`, with `selected` selected.
    public init(bots: BotListStore, projects: ProjectsStore, selected: SidebarSelection? = nil) {
        self.bots = bots
        self.projects = projects
        self.selected = selected
    }

    /// The selected bot, if a bot is selected and still on the team.
    public var selectedBot: Hearth_V1_Bot? {
        guard case .bot(let id) = selected else { return nil }
        return bots.bots.first { $0.id == id }
    }

    /// The selected project, if a project is selected and still listed.
    public var selectedProject: Hearth_V1_Project? {
        guard case .project(let id) = selected else { return nil }
        return projects.project(id: id)
    }

    /// Confirms a proposal and selects the project it became.
    public func confirm(_ proposal: Hearth_V1_Project) async {
        if let project = await projects.confirm(proposal) { selected = .project(project.id) }
    }

    /// Selects what holds the conversation `conversationID`: the bot whose
    /// DM it is, else the project whose conversation it is, else the Inbox.
    public func show(conversationID: String) {
        if let bot = bots.bots.first(where: { $0.dmConversationID == conversationID }) {
            selected = .bot(bot.id)
        } else if let project = projects.projects.first(where: {
            $0.conversationID == conversationID
        }) {
            selected = .project(project.id)
        } else {
            selected = .inbox
        }
    }

    /// Selects the first org-level bot, the Chief of Staff, when nothing is
    /// selected; a selection the owner made stays.
    public func selectOfficeIfNothingSelected() {
        guard selected == nil, let first = bots.office.first else { return }
        selected = .bot(first.id)
    }
}
