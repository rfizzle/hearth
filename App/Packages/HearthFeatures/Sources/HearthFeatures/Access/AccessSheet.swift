// AccessSheet shows what one bot can see and do: a heading and a lead, a row
// for each capability with its plain name, what it lets the bot do and a
// segmented control, then the folders the file capabilities reach, the bot's
// own fixed and others added through the system's folder panel, then a
// collapsed Advanced disclosure with the model pinned for this bot and its job
// description as the core keeps it. The rows are
// controls, not text fields, so they sit in a plain stack separated by
// hairlines rather than a grouped form. The whole body scrolls, and Cancel and
// the one prominent Save float on a scroll edge at the bottom, as the hire
// sheet's do. The sheet is a function of store state; it reports a saved
// change or a cancel to its caller and has no state of its own.
// See docs/design.md#forms-sheets-and-settings

import HearthDesign
import HearthKit
import HearthUI
import SwiftUI

/// The sheet that edits one bot's grants.
public struct AccessSheet: View {
    let store: GrantsStore
    let advanced: BotModelStore
    let botName: String
    let onDone: () -> Void

    @State private var showsAdvanced: Bool

    @Environment(\.pickFolder) private var pickFolder
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    /// `onDone` runs after a Save the core accepted and after Cancel.
    /// Advanced starts collapsed unless `advancedOpen`.
    public init(
        store: GrantsStore, advanced: BotModelStore, botName: String,
        advancedOpen: Bool = false, onDone: @escaping () -> Void
    ) {
        self.store = store
        self.advanced = advanced
        _showsAdvanced = State(initialValue: advancedOpen)
        self.botName = botName
        self.onDone = onDone
    }

    public var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: Theme.Spacing.lg) {
                VStack(alignment: .leading, spacing: Theme.Spacing.xs) {
                    Text("What \(botName) can see and do").font(Theme.title)
                    Text("Reading is always allowed; acting asks you unless you say otherwise.")
                        .font(Theme.body)
                        .foregroundStyle(Theme.color(.textSecondary))
                }
                capabilities
                foldersSection
                advancedSection
                if let error = store.error ?? advanced.error {
                    Label(error, systemImage: "exclamationmark.triangle")
                        .foregroundStyle(Theme.color(.toneNegative))
                }
            }
            .padding(.horizontal, Theme.Spacing.xl)
            .padding(.top, Theme.Spacing.xl)
        }
        .scrollEdgeEffectStyle(.soft, for: .bottom)
        .safeAreaBar(edge: .bottom) { buttons }
        .frame(width: Theme.Size.sheetWidth)
        .presentationSizing(.form)
        .task {
            await store.load()
            await advanced.load()
        }
    }

    private var capabilities: some View {
        VStack(spacing: 0) {
            ForEach(store.rows) { row in
                if row.id != store.rows.first?.id { Divider() }
                AccessRow(
                    symbol: row.capability.symbol, title: row.title,
                    summary: row.capability.summary, canAllow: row.canAllow,
                    mode: Binding(
                        get: { store.mode(for: row.capability) },
                        set: { store.set(mode: $0, for: row.capability) }))
            }
        }
    }

    private var foldersSection: some View {
        VStack(alignment: .leading, spacing: Theme.Spacing.sm) {
            Text("Folders").font(Theme.headline)
            Text("Reading, writing and deleting reach these folders, and no others.")
                .font(Theme.caption)
                .foregroundStyle(Theme.color(.textSecondary))
            VStack(spacing: 0) {
                ForEach(store.folders) { folder in
                    if folder.id != store.folders.first?.id { Divider() }
                    FolderRow(
                        path: folder.path, isOwn: folder.isOwn,
                        onRemove: { store.removeFolder(folder.path) })
                }
            }
            .animation(Theme.motion(Theme.move, reduceMotion: reduceMotion), value: store.folders)
            Button("Add Folder…") {
                Task {
                    if let path = await pickFolder() { store.addFolder(path) }
                }
            }
        }
    }

    private var advancedSection: some View {
        DisclosureGroup("Advanced", isExpanded: $showsAdvanced) {
            VStack(alignment: .leading, spacing: Theme.Spacing.md) {
                VStack(alignment: .leading, spacing: Theme.Spacing.xs) {
                    Text("Model").font(Theme.headline)
                    ModelOptionPicker(
                        options: advanced.options,
                        pinned: Binding(get: { advanced.pinned }, set: { advanced.pinned = $0 }))
                    Text("This bot uses this model whatever its tier says.")
                        .font(Theme.caption)
                        .foregroundStyle(Theme.color(.textSecondary))
                }
                VStack(alignment: .leading, spacing: Theme.Spacing.xs) {
                    Text("Job description").font(Theme.headline)
                    Text(advanced.jobDescription)
                        .font(Theme.monospaced)
                        .textSelection(.enabled)
                        .frame(maxWidth: .infinity, alignment: .leading)
                        .padding(Theme.Spacing.md)
                        .background(
                            Theme.color(.surfaceSecondary),
                            in: RoundedRectangle(cornerRadius: Theme.Radius.md))
                }
            }
            .padding(.top, Theme.Spacing.sm)
        }
        .font(Theme.headline)
        .animation(Theme.motion(Theme.move, reduceMotion: reduceMotion), value: showsAdvanced)
    }

    private var buttons: some View {
        HStack {
            Button("Cancel") {
                store.discard()
                advanced.discard()
                onDone()
            }
            .keyboardShortcut(.cancelAction)
            Spacer()
            Button("Save") { Task { await save() } }
                .buttonStyle(.borderedProminent)
                .keyboardShortcut(.defaultAction)
                .disabled(store.isSaving || advanced.isSaving)
        }
        .padding(.horizontal, Theme.Spacing.xl)
        .padding(.vertical, Theme.Spacing.md)
    }

    private func save() async {
        await store.save()
        guard store.didSave else { return }
        await advanced.save()
        if advanced.didSave { onDone() }
    }
}

extension Hearth_V1_Capability {
    /// The symbol that stands for the capability.
    var symbol: String {
        switch self {
        case .filesRead: "doc.text"
        case .filesWrite: "doc.badge.plus"
        case .filesDelete: "trash"
        case .webFetch: "globe"
        case .webSearch: "magnifyingglass"
        default: "questionmark.circle"
        }
    }

    /// What the capability lets the bot do, in a line.
    var summary: String {
        switch self {
        case .filesRead: "Open and read files in the folders below."
        case .filesWrite: "Create and edit files in the folders below."
        case .filesDelete: "Delete files in the folders below."
        case .webFetch: "Open a web page and read it."
        case .webSearch: "Search the web for an answer."
        default: ""
        }
    }
}
