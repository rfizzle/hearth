// SidebarList is the window's sidebar, one List with one selection: given the
// Inbox, a row at the top opens it, carrying how many items are open in a Pill
// in the attention tone and nothing when none are (its label says the count
// too); then the bots' sections from BotList, the Office and "Your team"; then
// "Projects", one ProjectRow per active project and the done ones after them.
// Archived projects are not listed. With no project, once the list has
// loaded, the section offers one line and four ExampleCards when the window
// gives it onExample, and shows the header alone otherwise, as "Your team"
// does with nobody hired, so the owner learns the shape.
// A project's context menu offers Edit Project… when the window gives it, Mark
// as Done while it is active, Archive, and Show Folder in Finder through the
// revealInFinder action. A new bot or project slides into the list under the
// move spring; nothing animates under Reduce Motion.
// See docs/design.md#window-anatomy, docs/design.md#motion and
// docs/model.md#work

import HearthDesign
import HearthKit
import HearthUI
import SwiftUI

/// The sidebar: the Inbox, the bots and the projects.
public struct SidebarList: View {
    let store: SidebarStore
    let inbox: InboxStore?
    let onAccess: ((Hearth_V1_Bot) -> Void)?
    let onEditProject: ((Hearth_V1_Project) -> Void)?
    let onExample: ((ProjectExample) -> Void)?

    @Environment(\.revealInFinder) private var revealInFinder
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    /// The sidebar over `store`, with the Inbox's row when `inbox` is given.
    /// A bot's context menu offers Access… when `onAccess` is given, and a
    /// project's offers Edit Project… when `onEditProject` is. With no project
    /// the examples start `onExample`.
    public init(
        store: SidebarStore, inbox: InboxStore? = nil,
        onAccess: ((Hearth_V1_Bot) -> Void)? = nil,
        onEditProject: ((Hearth_V1_Project) -> Void)? = nil,
        onExample: ((ProjectExample) -> Void)? = nil
    ) {
        self.store = store
        self.inbox = inbox
        self.onAccess = onAccess
        self.onEditProject = onEditProject
        self.onExample = onExample
    }

    public var body: some View {
        @Bindable var store = store
        List(selection: $store.selected) {
            if let inbox {
                Section {
                    InboxRow(openCount: inbox.openCount)
                        .tag(SidebarSelection.inbox)
                }
            }
            BotList(store: store.bots, onAccess: onAccess)
            Section("Projects") {
                ForEach(store.projects.listed, id: \.id) { row($0) }
                if let onExample, store.projects.loaded, store.projects.listed.isEmpty {
                    examples(onExample)
                }
            }
        }
        .listStyle(.sidebar)
        .animation(
            Theme.motion(Theme.move, reduceMotion: reduceMotion), value: store.bots.bots.map(\.id)
        )
        .animation(
            Theme.motion(Theme.move, reduceMotion: reduceMotion),
            value: store.projects.listed.map(\.id)
        )
        .task { await store.bots.load() }
        .task { await store.projects.load() }
        .task { await inbox?.load() }
    }

    @ViewBuilder
    private func examples(_ onExample: @escaping (ProjectExample) -> Void) -> some View {
        Text("Start with an idea, or choose New Project.")
            .font(Theme.callout)
            .foregroundStyle(Theme.color(.textSecondary))
            .lineLimit(nil)
            .fixedSize(horizontal: false, vertical: true)
            .listRowSeparator(.hidden)
        ForEach(ProjectExample.all) { example in
            ExampleCard(symbol: example.symbol, title: example.title, line: example.line) {
                onExample(example)
            }
            .listRowSeparator(.hidden)
        }
    }

    private func row(_ project: Hearth_V1_Project) -> some View {
        ProjectRow(project: project)
            .tag(SidebarSelection.project(project.id))
            .contextMenu {
                if let onEditProject {
                    Button("Edit Project…", systemImage: "pencil") { onEditProject(project) }
                }
                if project.state == .active {
                    Button("Mark as Done", systemImage: "checkmark.circle") {
                        Task { await store.projects.markDone(id: project.id) }
                    }
                }
                Button("Archive", systemImage: "archivebox") {
                    Task { await store.projects.archive(id: project.id) }
                }
                Divider()
                Button("Show Folder in Finder", systemImage: "folder") {
                    revealInFinder(project.folderPath)
                }
                .disabled(project.folderPath.isEmpty)
            }
    }
}

/// The sidebar's row for the Inbox, with the open count.
struct InboxRow: View {
    let openCount: Int

    var body: some View {
        HStack(spacing: Theme.Spacing.sm) {
            Label("Inbox", systemImage: "tray")
                .font(Theme.body)
            Spacer(minLength: Theme.Spacing.sm)
            if let count = Self.count(openCount) {
                Pill(count, tone: .attention)
            }
        }
        .accessibilityElement(children: .ignore)
        .accessibilityLabel("Inbox")
        .accessibilityValue(Self.spoken(openCount))
    }

    /// The count the row shows, or nil when nothing is open.
    static func count(_ openCount: Int) -> String? {
        openCount > 0 ? String(openCount) : nil
    }

    /// The count as VoiceOver says it.
    static func spoken(_ openCount: Int) -> String {
        switch openCount {
        case 0: "Nothing waiting"
        case 1: "1 item waiting"
        default: "\(openCount) items waiting"
        }
    }
}
