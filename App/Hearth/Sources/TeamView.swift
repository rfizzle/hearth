// TeamView is the window once the org exists: the Inbox, the bots and the
// projects in the sidebar and the selected one in the detail column, with
// Hire Someone… and New Project… as icon-only toolbar items over the sidebar,
// and the bot's folder as the one item beside Access… and the inspector toggle
// over a bot's conversation. The inspector opens beside it with what the bot
// remembers. The Inbox row opens the Inbox in the detail column. A project
// opens its own conversation with its brief pinned, titled by the project's
// name and answered by the Chief of Staff; with no Chief of Staff on the team
// it says so instead. A project the Chief of Staff proposed is a card in the
// conversation it was proposed in, and creating it selects it. A project's
// toolbar has its folder and a menu to edit, mark done or archive it.
// Nobody on the team shows the invitation in the detail column, with an
// example card for each role template that opens the hire setup with it
// picked; no project shows example cards in the sidebar that open the project
// setup with a paragraph filled in; nothing
// selected says to choose. The Access and setup sheets are presented here (a
// hire and a new project are the Chief of Staff's guided setup, a project's
// edit its form), and the helper notice sits over the top of the detail column
// until the owner dismisses it. A new project or bot is selected once created.
// When the window first shows the team with nothing selected, the Chief of
// Staff is.
// See docs/design.md#window-anatomy and docs/model.md#9-failure-handling

import HearthDesign
import HearthFeatures
import HearthKit
import HearthUI
import SwiftUI

struct TeamView: View {
    let session: Session
    @Environment(AppModel.self) private var model
    @Environment(\.revealInFinder) private var revealInFinder
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    /// The role templates the empty team's cards are drawn from.
    @State private var templates: [Hearth_V1_RoleTemplate] = []

    var body: some View {
        @Bindable var model = model
        NavigationSplitView {
            SidebarList(
                store: session.sidebar, inbox: session.inbox,
                onAccess: { model.accessBotID = $0.id },
                onEditProject: { model.editingProjectID = $0.id },
                onExample: {
                    model.projectOpening = $0.paragraph
                    model.isCreatingProject = true
                }
            )
            .navigationSplitViewColumnWidth(
                min: Theme.Size.sidebarMin, ideal: Theme.Size.sidebarIdeal,
                max: Theme.Size.sidebarMax
            )
            .toolbar {
                ToolbarItem(placement: .primaryAction) {
                    Button("Hire Someone…", systemImage: "person.badge.plus") {
                        model.isHiring = true
                    }
                    .labelStyle(.iconOnly)
                    .help("Hire someone")
                }
                ToolbarItem(placement: .primaryAction) {
                    Button("New Project…", systemImage: "folder.badge.plus") {
                        model.isCreatingProject = true
                    }
                    .labelStyle(.iconOnly)
                    .help("New project")
                }
            }
        } detail: {
            detail
                .safeAreaInset(edge: .top, spacing: 0) {
                    if model.helperRestarted {
                        HelperBanner(onDismiss: model.dismissHelperNotice)
                            .transition(.move(edge: .top).combined(with: .opacity))
                    }
                }
                .animation(
                    Theme.motion(Theme.quick, reduceMotion: reduceMotion),
                    value: model.helperRestarted)
        }
        .navigationSplitViewStyle(.balanced)
        .onChange(of: session.bots.office.first?.id, initial: true) {
            session.sidebar.selectOfficeIfNothingSelected()
        }
        .task { await session.usage.load() }
        .task { templates = (try? await session.client.listTemplates()) ?? [] }
        .sheet(
            isPresented: $model.isHiring, onDismiss: { model.hireTemplate = nil },
            content: {
                SetupSheetHost(
                    session: session, script: .hire, editing: nil, template: model.hireTemplate)
            }
        )
        .sheet(isPresented: isChangingAccess) {
            if let bot = accessBot { AccessSheetHost(session: session, bot: bot) }
        }
        .sheet(
            isPresented: $model.isCreatingProject, onDismiss: { model.projectOpening = nil },
            content: {
                SetupSheetHost(
                    session: session, script: .project, editing: nil,
                    opening: model.projectOpening)
            }
        )
        .sheet(isPresented: isEditingProject) {
            if let project = editingProject {
                SetupSheetHost(session: session, script: .project, editing: project)
            }
        }
    }

    /// Whether the edit sheet is open for a project still listed.
    private var isEditingProject: Binding<Bool> {
        Binding(
            get: { editingProject != nil },
            set: { if !$0 { model.editingProjectID = nil } })
    }

    private var editingProject: Hearth_V1_Project? {
        model.editingProjectID.flatMap { session.projects.project(id: $0) }
    }

    /// Whether the Access sheet is open for a bot still on the team.
    private var isChangingAccess: Binding<Bool> {
        Binding(
            get: { accessBot != nil },
            set: { if !$0 { model.accessBotID = nil } })
    }

    private var accessBot: Hearth_V1_Bot? {
        session.bots.bots.first { $0.id == model.accessBotID }
    }

    @ViewBuilder
    private var detail: some View {
        @Bindable var model = model
        if session.sidebar.selected == .inbox {
            InboxScreen(store: session.inbox, bots: session.bots.bots)
        } else if let project = session.sidebar.selectedProject {
            projectDetail(project)
        } else if let bot = session.sidebar.selectedBot {
            ConversationScreen(
                store: session.conversation(for: bot.dmConversationID, botID: bot.id), bot: bot,
                proposals: session.projects.proposals,
                onCreateProposal: { proposal in
                    Task { await session.sidebar.confirm(proposal) }
                },
                onDismissProposal: { proposal in
                    Task { await session.projects.dismiss(proposal) }
                }
            )
            .id(bot.id)
            .navigationSubtitle(bot.role)
            .inspector(isPresented: $model.showsInspector) {
                InspectorView(
                    notes: session.notes(for: bot), handbook: session.handbook(for: bot),
                    usage: session.usage, botName: bot.name
                )
                .inspectorColumnWidth(
                    min: Theme.Size.inspectorMin, ideal: Theme.Size.inspectorIdeal,
                    max: Theme.Size.inspectorMax)
            }
            .toolbar {
                ToolbarItem(placement: .primaryAction) {
                    Button("Access…", systemImage: "lock.open") {
                        model.accessBotID = bot.id
                    }
                    .labelStyle(.iconOnly)
                    .help("What \(bot.name) can see and do")
                }
                ToolbarItem(placement: .primaryAction) {
                    Button("Show Folder in Finder", systemImage: "folder") {
                        revealInFinder(bot.folderPath)
                    }
                    .labelStyle(.iconOnly)
                    .help("Show \(bot.name)'s folder in Finder")
                }
                ToolbarItem(placement: .primaryAction) {
                    Button("Show Inspector", systemImage: "sidebar.trailing") {
                        model.showsInspector.toggle()
                    }
                    .labelStyle(.iconOnly)
                    .help("Show or hide what \(bot.name) remembers")
                }
            }
        } else if session.bots.showsHireAffordance {
            TeamEmptyView(
                templates: templates, onHire: { model.isHiring = true },
                onPick: {
                    model.hireTemplate = $0
                    model.isHiring = true
                }
            )
            .navigationTitle(session.org?.name ?? "Hearth")
        } else {
            ContentUnavailableView(
                "No one selected", systemImage: "bubble.left.and.bubble.right",
                description: Text("Choose someone in the sidebar.")
            )
            .navigationTitle(session.org?.name ?? "Hearth")
        }
    }
}

extension TeamView {
    /// A project's conversation, answered by the Chief of Staff.
    fileprivate func projectDetail(_ project: Hearth_V1_Project) -> some View {
        ConversationScreen(
            store: session.conversation(
                for: project.conversationID, botID: session.coordinator?.id),
            project: project, coordinator: session.coordinator,
            proposals: session.projects.proposals,
            onCreateProposal: { proposal in
                Task { await session.sidebar.confirm(proposal) }
            },
            onDismissProposal: { proposal in
                Task { await session.projects.dismiss(proposal) }
            },
            onMarkDone: { Task { await session.projects.markDone(id: project.id) } },
            onHire: { model.isHiring = true }
        )
        .id(project.conversationID)
        .toolbar {
            ToolbarItem(placement: .primaryAction) {
                Button("Show Folder in Finder", systemImage: "folder") {
                    revealInFinder(project.folderPath)
                }
                .labelStyle(.iconOnly)
                .help("Show the project's folder in Finder")
            }
            ToolbarItem(placement: .primaryAction) {
                Menu("Project", systemImage: "ellipsis.circle") {
                    Button("Edit Project…") { model.editingProjectID = project.id }
                    Button("Mark as Done") {
                        Task { await session.projects.markDone(id: project.id) }
                    }
                    .disabled(project.state != .active)
                    Button("Archive") {
                        Task { await session.projects.archive(id: project.id) }
                    }
                }
                .labelStyle(.iconOnly)
                .help("Edit, finish or archive the project")
            }
        }
    }
}

/// One presentation of a setup sheet, a hire, a new project or an edit, over a
/// store of its own each time, asked by the Chief of Staff; a new project or
/// bot is selected once the core has made it. Until the accounts have loaded the sheet assumes a model
/// may draft, and the core's `unavailable` says otherwise.
private struct SetupSheetHost: View {
    let session: Session
    let editing: Hearth_V1_Project?
    @Environment(AppModel.self) private var model
    @State private var store: SetupStore

    /// A presentation of `script`; `opening` starts the opener's box with that
    /// paragraph and `template` with that role picked, as an example card
    /// does. With neither it opens as it always has.
    init(
        session: Session, script: SetupScript, editing: Hearth_V1_Project?,
        opening: String? = nil, template: Hearth_V1_RoleTemplate? = nil
    ) {
        self.session = session
        self.editing = editing
        _store = State(
            initialValue: SetupStore(
                script: script, client: session.client, projects: session.projects,
                canDraft: Self.canDraft(session), editing: editing,
                opening: opening, template: template))
    }

    var body: some View {
        SetupSheet(
            store: store, chief: session.coordinator,
            onDone: { outcome in
                switch outcome {
                case .project(let project):
                    if editing == nil { session.sidebar.selected = .project(project.id) }
                case .bot(let bot):
                    session.sidebar.selected = .bot(bot.id)
                }
                close()
            },
            onCancel: close
        )
        .onAppear { session.projects.clearSaveError() }
        .task { await session.accounts.load() }
        .onChange(of: Self.canDraft(session)) { _, canDraft in store.canDraft = canDraft }
    }

    private static func canDraft(_ session: Session) -> Bool {
        session.accounts.accounts.isEmpty || session.accounts.hasSignedInAccount
    }

    private func close() {
        model.isHiring = false
        model.isCreatingProject = false
        model.editingProjectID = nil
    }
}

/// One presentation of the Access sheet, over a grants store made for it.
private struct AccessSheetHost: View {
    let bot: Hearth_V1_Bot
    @Environment(AppModel.self) private var model
    @State private var store: GrantsStore
    @State private var advanced: BotModelStore

    init(session: Session, bot: Hearth_V1_Bot) {
        self.bot = bot
        _store = State(
            initialValue: GrantsStore(
                client: session.client, relay: session.relay, botID: bot.id))
        _advanced = State(
            initialValue: BotModelStore(
                client: session.client, models: session.models, bot: bot))
    }

    var body: some View {
        AccessSheet(
            store: store, advanced: advanced, botName: bot.name, onDone: { model.accessBotID = nil }
        )
    }
}
