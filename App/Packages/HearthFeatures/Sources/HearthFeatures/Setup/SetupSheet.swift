// SetupSheet is New Project as the Chief of Staff asks it: one step on screen
// at a time, from the script the store follows. Every guided step opens with
// a row of the Chief of Staff's small avatar (hidden from VoiceOver) and name,
// with where the owner is in words at its end, then the question as the
// sheet's heading in title type and its second line. The opener's box is the
// question card's editor with the script's example as its placeholder;
// Return there presses Draft and Shift-Return starts a line, the composer's
// rule, because the box is a message to a person. The hire script's opener
// has a chip for each role template under the box. An asking step is the
// question card's control for one field with an example under it. The card
// (SetupSheet+Card) holds every field. The quiet line saying why there is no
// draft sits under the box and arrives without animation. "Just show me the
// form" sits under every guided step; the form carries the way back. With no
// Chief of Staff the row is the script's title and no avatar. The bar holds
// Cancel (Escape), Back from the second step on, and the one prominent
// button on Return, with a small spinner beside it while a call is in flight
// so its label never changes width. Steps cross-fade while the column's
// height moves with them; under Reduce Motion the cross-fade stays, quick, so
// the owner keeps their place. 560 points wide, no fixed height, on a scroll
// edge.
// See docs/design.md#forms-sheets-and-settings, docs/design.md#motion and
// docs/model.md#owner-interaction

import HearthDesign
import HearthKit
import HearthUI
import SwiftUI

/// The guided sheet that sets up a project or a hire, or the form that edits a project.
public struct SetupSheet: View {
    let store: SetupStore
    let chief: Hearth_V1_Bot?
    let onDone: (SetupStore.Outcome) -> Void
    let onCancel: () -> Void

    @State var closedGroups: Set<String> = []
    @Environment(\.accessibilityReduceMotion) var reduceMotion

    /// A sheet over `store`, asked by `chief` (the Chief of Staff, if the
    /// org has one); `onDone` takes what Create or Save made.
    public init(
        store: SetupStore, chief: Hearth_V1_Bot?,
        onDone: @escaping (SetupStore.Outcome) -> Void, onCancel: @escaping () -> Void
    ) {
        self.store = store
        self.chief = chief
        self.onDone = onDone
        self.onCancel = onCancel
    }

    public var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: Theme.Spacing.lg) {
                switch store.step {
                case .opener: opener.transition(.opacity)
                case .asking(let id):
                    if let field = store.script.field(id) {
                        asking(field).transition(.opacity)
                    }
                case .card: card.transition(.opacity.animation(quick))
                case .form: form.transition(.opacity.animation(quick))
                }
            }
            .padding(.top, Theme.Spacing.xl)
            .padding(.bottom, Theme.Spacing.lg)
            .animation(stepChange, value: store.step)
            .geometryGroup()
        }
        .scrollEdgeEffectStyle(.soft, for: .bottom)
        .safeAreaBar(edge: .bottom) { buttons }
        .frame(width: Theme.Size.sheetWidth)
        .presentationSizing(.form)
        .task { await store.load() }
    }

    /// The step change: the layout move, or under Reduce Motion the quick
    /// fade alone, so a step never jumps.
    private var stepChange: Animation {
        Theme.motion(Theme.move, reduceMotion: reduceMotion) ?? Theme.quick
    }

    var quick: Animation? {
        Theme.motion(Theme.quick, reduceMotion: reduceMotion)
    }

    /// The Chief of Staff's row over every guided step.
    var header: some View {
        HStack(spacing: Theme.Spacing.sm) {
            if let chief {
                Avatar(bot: chief).accessibilityHidden(true)
                Text(chief.name)
            } else {
                Text(store.script.title)
            }
            Spacer(minLength: Theme.Spacing.sm)
            if let progress = store.progress {
                Text(progress).font(Theme.caption)
            }
        }
        .font(Theme.callout)
        .foregroundStyle(Theme.color(.textSecondary))
    }

    /// A question as the step's heading, with its second line.
    func question(_ text: String, _ secondLine: String) -> some View {
        VStack(alignment: .leading, spacing: Theme.Spacing.xs) {
            Text(text)
                .font(Theme.title)
                .accessibilityAddTraits(.isHeader)
            Text(secondLine)
                .font(Theme.body)
                .foregroundStyle(Theme.color(.textSecondary))
        }
        .fixedSize(horizontal: false, vertical: true)
    }

    private var opener: some View {
        let opener = store.script.opener
        return VStack(alignment: .leading, spacing: Theme.Spacing.lg) {
            header
            question(opener.question, opener.secondLine)
            WidgetControl(widget: openerWidget, draft: paragraph, label: opener.question)
                .submitOnReturn { Task { await primary() } }
            starters
            if let error = store.error {
                Text(error)
                    .font(Theme.body)
                    .foregroundStyle(Theme.color(.toneNegative))
            }
            if let line = store.fallbackLine {
                Text(line)
                    .font(Theme.caption)
                    .foregroundStyle(Theme.color(.textSecondary))
                    .fixedSize(horizontal: false, vertical: true)
                    .transaction { $0.animation = nil }
            }
            formLink
        }
        .padding(.horizontal, Theme.Spacing.xl)
    }

    /// The hire opener's chips, one per role template: a press puts the
    /// role's description in the box and never opens anything.
    @ViewBuilder
    private var starters: some View {
        if !store.templates.isEmpty {
            ViewThatFits(in: .horizontal) {
                HStack(spacing: Theme.Spacing.sm) { chips }
                VStack(alignment: .leading, spacing: Theme.Spacing.sm) { chips }
            }
        }
    }

    private var chips: some View {
        ForEach(store.templates, id: \.id) { template in
            SuggestionChip(template.role, hint: "Puts this role\u{2019}s description in the box") {
                store.pick(template)
            }
        }
    }

    /// The opener's box: the question card's editor, at the core's bound for
    /// a description, with the example as its placeholder.
    private var openerWidget: Hearth_V1_Widget {
        .with {
            $0.freeText = .with {
                $0.multiline = true
                $0.maxLength = 4000
                $0.placeholder = store.script.opener.example
            }
        }
    }

    private var paragraph: Binding<WidgetDraft> {
        Binding {
            var draft = WidgetDraft()
            draft.text = store.paragraph
            return draft
        } set: {
            store.paragraph = $0.text
        }
    }

    private func asking(_ field: SetupScript.Field) -> some View {
        VStack(alignment: .leading, spacing: Theme.Spacing.lg) {
            header
            question(field.question, field.secondLine)
            VStack(alignment: .leading, spacing: Theme.Spacing.xs) {
                WidgetControl(widget: field.widget, draft: draft(field.id), label: field.label)
                Text("For example: \u{201C}\(field.example)\u{201D}")
                    .font(Theme.caption)
                    .foregroundStyle(Theme.color(.textSecondary))
                    .fixedSize(horizontal: false, vertical: true)
            }
            formLink
        }
        .padding(.horizontal, Theme.Spacing.xl)
    }

    /// The field with `id`'s value, as a control edits it.
    func draft(_ id: String) -> Binding<WidgetDraft> {
        Binding {
            store.drafts[id] ?? WidgetDraft()
        } set: {
            store.drafts[id] = $0
        }
    }

    private var formLink: some View {
        Button("Just show me the form") { store.showForm() }
            .buttonStyle(.link)
            .font(Theme.callout)
            .disabled(store.isWorking)
    }

    private var buttons: some View {
        HStack(spacing: Theme.Spacing.sm) {
            Button("Cancel", action: onCancel)
                .keyboardShortcut(.cancelAction)
            if store.canGoBack {
                Button("Back") { store.back() }.disabled(store.isWorking)
            }
            Spacer()
            if store.isWorking {
                ProgressView().controlSize(.small)
            }
            Button(store.primaryLabel) { Task { await primary() } }
                .buttonStyle(.borderedProminent)
                .keyboardShortcut(.defaultAction)
                .disabled(!store.canContinue)
        }
        .padding(.horizontal, Theme.Spacing.xl)
        .padding(.vertical, Theme.Spacing.md)
    }

    private func primary() async {
        await store.primary()
        if let outcome = store.outcome { onDone(outcome) }
    }
}
