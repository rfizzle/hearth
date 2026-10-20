// The setup sheet's last guided step and its form. The card holds every
// field of the script on a card surface in card order, each a caption label
// over the question card's control for it, so a project reads in the
// project header's order: the name, the "done when" sentence, the Brief
// disclosure (open) over its three editors, then the optional target date as
// a switch with a date picker; a hire's reads like a row of the team list, a
// live avatar beside the name and the role, then the job description, the
// quality as a choice of three and the avatar text. Each of the core's concerns sits under the
// field it names and leaves under the quick fade when it is settled, while
// the card's height moves; one that names no field sits at the card's end.
// The card is one VoiceOver container named "Your project". The core's
// refusal of Create shows under the card in its words. The form is the old
// sheet's: the title and lead, then a grouped form of the same controls with
// their labels, the date section and the refusal; editing opens here, and a
// new project's form carries the way back to the guided steps.
// See docs/design.md#forms-sheets-and-settings and docs/design.md#motion

import HearthDesign
import HearthKit
import HearthUI
import SwiftUI

/// A run of the card: a field alone, a disclosure row over several, or the
/// fields drawn beside the live avatar.
enum CardSection: Identifiable {
    case field(SetupScript.Field)
    case group(String, [SetupScript.Field])
    case identity([SetupScript.Field])

    var id: String {
        switch self {
        case .field(let field): field.id
        case .group(let name, _): "group-" + name
        case .identity: "identity"
        }
    }

    /// The script's fields in card order, a group's fields gathered where
    /// its first one stands.
    static func sections(_ fields: [SetupScript.Field], paired: [String] = []) -> [CardSection] {
        var sections: [CardSection] = []
        for field in fields {
            if paired.contains(field.id) {
                if !sections.contains(where: { $0.id == "identity" }) {
                    sections.append(.identity(fields.filter { paired.contains($0.id) }))
                }
                continue
            }
            guard let group = field.group else {
                sections.append(.field(field))
                continue
            }
            if sections.contains(where: { $0.id == "group-" + group }) { continue }
            sections.append(.group(group, fields.filter { $0.group == group }))
        }
        return sections
    }
}

extension SetupSheet {
    var card: some View {
        VStack(alignment: .leading, spacing: Theme.Spacing.lg) {
            header
            Text(store.script.cardLine)
                .font(Theme.headline)
                .accessibilityAddTraits(.isHeader)
            CardSurface {
                VStack(alignment: .leading, spacing: Theme.Spacing.md) {
                    ForEach(
                        CardSection.sections(
                            store.script.fields, paired: store.script.pairedWithAvatar)
                    ) { section in
                        switch section {
                        case .identity(let fields): identity(fields)
                        case .field(let field): cell(field)
                        case .group(let name, let fields): group(name, fields)
                        }
                    }
                    ForEach(store.looseConcerns, id: \.self) { concern in
                        concernRow(concern, label: store.script.title)
                    }
                }
                .animation(
                    Theme.motion(Theme.move, reduceMotion: reduceMotion), value: store.concerns)
            }
            .accessibilityElement(children: .contain)
            .accessibilityLabel(store.script.cardLabel)
            refusal
        }
        .padding(.horizontal, Theme.Spacing.xl)
    }

    /// The avatar the bot would have, live, beside the fields that name it.
    private func identity(_ fields: [SetupScript.Field]) -> some View {
        HStack(alignment: .top, spacing: Theme.Spacing.md) {
            Avatar(
                name: store.drafts["name"]?.text ?? "",
                avatar: store.drafts["avatar"]?.text ?? "", size: .large
            )
            .accessibilityHidden(true)
            VStack(alignment: .leading, spacing: Theme.Spacing.md) {
                ForEach(fields) { cell($0) }
            }
        }
    }

    private func group(_ name: String, _ fields: [SetupScript.Field]) -> some View {
        DisclosureRow(
            isExpanded: Binding {
                !closedGroups.contains(name)
            } set: {
                if $0 { closedGroups.remove(name) } else { closedGroups.insert(name) }
            },
            label: { Text(name) },
            content: {
                VStack(alignment: .leading, spacing: Theme.Spacing.md) {
                    ForEach(fields) { cell($0) }
                }
                .padding(.bottom, Theme.Spacing.sm)
            })
    }

    /// One field on the card: its label over its control, then its concerns.
    private func cell(_ field: SetupScript.Field) -> some View {
        VStack(alignment: .leading, spacing: Theme.Spacing.sm) {
            if isOptionalDay(field) {
                optionalDay(field)
            } else {
                Text(field.label)
                    .font(Theme.caption)
                    .foregroundStyle(Theme.color(.textSecondary))
                    .accessibilityHidden(true)
                WidgetControl(widget: field.widget, draft: draft(field.id), label: field.label)
            }
            ForEach(store.concerns(for: field.id), id: \.self) { concern in
                concernRow(concern, label: field.label)
            }
        }
    }

    private func concernRow(_ concern: Hearth_V1_Concern, label: String) -> some View {
        ConcernRow(
            concern: concern, field: label, onUse: { store.use(concern) },
            onKeep: { store.keep(concern) }
        )
        .transition(.opacity.animation(quick))
    }

    /// Whether `field` is a day the owner may leave unset: a switch, then a
    /// picker, as the old sheet drew a target date.
    private func isOptionalDay(_ field: SetupScript.Field) -> Bool {
        if case .date = field.widget.kind { return !field.required }
        return false
    }

    @ViewBuilder
    private func optionalDay(_ field: SetupScript.Field) -> some View {
        let value = draft(field.id)
        Toggle(
            "Has a \(field.label.lowercased())",
            isOn: Binding {
                value.wrappedValue.date != nil
            } set: {
                value.wrappedValue.date = $0 ? Calendar.current.startOfDay(for: .now) : nil
            })
        if let date = value.wrappedValue.date {
            DatePicker(
                field.label,
                selection: Binding {
                    date
                } set: {
                    value.wrappedValue.date = $0
                },
                displayedComponents: .date
            )
            .transition(.opacity.animation(quick))
        }
    }

    @ViewBuilder
    private var refusal: some View {
        if let error = store.saveError {
            Text(error)
                .font(Theme.body)
                .foregroundStyle(Theme.color(.toneNegative))
                .fixedSize(horizontal: false, vertical: true)
        }
    }

    var form: some View {
        VStack(alignment: .leading, spacing: Theme.Spacing.lg) {
            VStack(alignment: .leading, spacing: Theme.Spacing.xs) {
                Text(store.editing?.name ?? store.script.title)
                    .font(Theme.title)
                    .accessibilityAddTraits(.isHeader)
                Text(store.script.lead)
                    .font(Theme.body)
                    .foregroundStyle(Theme.color(.textSecondary))
            }
            .padding(.horizontal, Theme.Spacing.xl)
            Form {
                Section {
                    ForEach(store.script.fields.filter { !isOptionalDay($0) }) { formRow($0) }
                }
                Section {
                    ForEach(store.script.fields.filter(isOptionalDay)) { optionalDay($0) }
                }
                if let error = store.saveError {
                    Section {
                        Text(error).foregroundStyle(Theme.color(.toneNegative))
                    }
                }
            }
            .formStyle(.grouped)
            .scrollDisabled(true)
            if store.editing == nil {
                Button("Let the Chief of Staff walk me through it") {
                    Task { await store.showGuide() }
                }
                .buttonStyle(.link)
                .font(Theme.callout)
                .padding(.horizontal, Theme.Spacing.xl)
            }
        }
    }

    /// Whether the form draws `field` under its label: an editor or a choice.
    private func isStacked(_ field: SetupScript.Field) -> Bool {
        switch field.widget.kind {
        case .freeText(let spec): spec.multiline
        case .singleChoice: true
        default: false
        }
    }

    /// A field in the form: a text field beside its label, or an editor or
    /// a choice under it.
    @ViewBuilder
    private func formRow(_ field: SetupScript.Field) -> some View {
        if isStacked(field) {
            VStack(alignment: .leading, spacing: Theme.Spacing.xs) {
                Text(field.label)
                WidgetControl(widget: field.widget, draft: draft(field.id), label: field.label)
            }
        } else {
            // The row draws the label; the field keeps it for VoiceOver.
            LabeledContent(field.label) {
                WidgetControl(widget: field.widget, draft: draft(field.id), label: field.label)
                    .labelsHidden()
            }
        }
    }
}
