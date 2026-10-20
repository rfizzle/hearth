// ProjectSheet starts a project or edits one, in the hire sheet's shape: a
// heading ("New Project", or the project's name when editing) and a lead, then
// a grouped form, because everything here is plain text input: the name, the
// goal, the context and what done looks like (each an editor on a card, three
// lines at least), the one "done when" sentence, and a target date the owner
// switches on before a date picker slides in. Cancel and the one prominent
// Create (Save when editing) float on a scroll edge at the bottom: Escape
// cancels, Return saves once the name and every part of the brief are typed,
// the one check made here. The core's refusal shows under the form in its own
// words. The draft is the sheet's own state; the store makes the call and the
// sheet hands the saved project to its caller.
// See docs/design.md#forms-sheets-and-settings and docs/model.md#work

import HearthDesign
import HearthKit
import SwiftUI

/// The sheet that creates or edits one project.
public struct ProjectSheet: View {
    let store: ProjectsStore
    let editing: Hearth_V1_Project?
    let onSaved: (Hearth_V1_Project) -> Void
    let onCancel: () -> Void

    @State private var form: ProjectForm
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    /// Three lines of body text, so each part of the brief has room.
    @ScaledMetric(relativeTo: .body) private var editorHeight: CGFloat = 60

    /// A sheet for a new project, or for `editing` when given; `form` starts
    /// it filled, as the gallery shows.
    public init(
        store: ProjectsStore, editing: Hearth_V1_Project? = nil, form: ProjectForm? = nil,
        onSaved: @escaping (Hearth_V1_Project) -> Void, onCancel: @escaping () -> Void
    ) {
        self.store = store
        self.editing = editing
        self.onSaved = onSaved
        self.onCancel = onCancel
        _form = State(
            initialValue: form ?? editing.map(ProjectForm.init(project:)) ?? ProjectForm())
    }

    public var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: Theme.Spacing.lg) {
                VStack(alignment: .leading, spacing: Theme.Spacing.xs) {
                    Text(editing?.name ?? "New Project").font(Theme.title)
                    Text("Say what done looks like and the Chief of Staff can tell when it is.")
                        .font(Theme.body)
                        .foregroundStyle(Theme.color(.textSecondary))
                }
                .padding(.horizontal, Theme.Spacing.xl)
                fields
            }
            .padding(.top, Theme.Spacing.xl)
        }
        .scrollEdgeEffectStyle(.soft, for: .bottom)
        .safeAreaBar(edge: .bottom) { buttons }
        .frame(width: Theme.Size.sheetWidth)
        .presentationSizing(.form)
    }

    private var fields: some View {
        Form {
            Section {
                TextField("Name", text: $form.name, prompt: Text("Launch the newsletter"))
                editor("Goal", text: $form.goal)
                editor("Context", text: $form.context)
                editor("What done looks like", text: $form.doneLooksLike)
                TextField(
                    "Done when", text: $form.doneWhen,
                    prompt: Text("The first issue has gone out to the list"))
            }
            Section {
                Toggle("Has a target date", isOn: $form.hasTargetDate)
                if form.hasTargetDate {
                    DatePicker(
                        "Target date", selection: $form.targetDate, displayedComponents: .date
                    )
                    .transition(.move(edge: .top).combined(with: .opacity))
                }
            }
            if let error = store.saveError {
                Section {
                    Text(error).foregroundStyle(Theme.color(.toneNegative))
                }
            }
        }
        .formStyle(.grouped)
        .scrollDisabled(true)
        .animation(
            Theme.motion(Theme.move, reduceMotion: reduceMotion), value: form.hasTargetDate)
    }

    private func editor(_ title: String, text: Binding<String>) -> some View {
        VStack(alignment: .leading, spacing: Theme.Spacing.xs) {
            Text(title)
            CardSurface {
                TextEditor(text: text)
                    .font(Theme.body)
                    .scrollContentBackground(.hidden)
                    .frame(minHeight: editorHeight)
                    .accessibilityLabel(title)
            }
        }
    }

    private var buttons: some View {
        HStack {
            Button("Cancel", action: onCancel)
                .keyboardShortcut(.cancelAction)
            Spacer()
            Button(editing == nil ? "Create" : "Save") { Task { await save() } }
                .buttonStyle(.borderedProminent)
                .keyboardShortcut(.defaultAction)
                .disabled(!form.canSave || store.isSaving)
        }
        .padding(.horizontal, Theme.Spacing.xl)
        .padding(.vertical, Theme.Spacing.md)
    }

    private func save() async {
        let saved =
            if let editing {
                await store.update(form.updateRequest(id: editing.id))
            } else {
                await store.create(form.createRequest)
            }
        if let saved { onSaved(saved) }
    }
}
