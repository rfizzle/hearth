// WidgetControl is the control a bot asked the owner to answer a question
// with, drawn as the system control for its kind so the keyboard and
// VoiceOver come with it: a radio-group Picker for a single choice (with an
// Other row that reveals a field when the bot allows one), a column of
// checkbox Toggles for a multiple choice with its bounds and the live count
// as a caption, a TextField (a TextEditor on a card when multiline) with a
// count near its limit, two bordered toggle buttons for yes or no with the
// chosen one tinted (Answer stays the card's one prominent button), a DatePicker
// (graphical for a day, a field with a time), a row with the path, the
// system's icon and Choose… through the pickFolder action, and a segmented
// Picker of a scale's steps with its end labels beneath. Each control keeps
// its system label for VoiceOver even where the label is not drawn; a text
// field is "Your answer" unless the caller names it. The owner's answer is a
// WidgetDraft binding, presentation state the card sends.
// See docs/design.md#the-transcript

import HearthDesign
import HearthKit
import SwiftUI

/// The control for one question's widget.
public struct WidgetControl: View {
    let widget: Hearth_V1_Widget
    @Binding var draft: WidgetDraft
    /// Whether the control shows an answer that can no longer change.
    var isLocked = false
    let label: String

    /// The control for `widget`, holding the owner's answer in `draft`; a
    /// text field or editor is called `label` for VoiceOver.
    public init(
        widget: Hearth_V1_Widget, draft: Binding<WidgetDraft>, label: String = "Your answer"
    ) {
        self.widget = widget
        _draft = draft
        self.label = label
    }

    public var body: some View {
        control
            .frame(maxWidth: .infinity, alignment: .leading)
            .disabled(isLocked)
    }

    @ViewBuilder
    private var control: some View {
        switch widget.kind {
        case .singleChoice(let spec): SingleChoiceControl(spec: spec, draft: $draft)
        case .multipleChoice(let spec): MultipleChoiceControl(spec: spec, draft: $draft)
        case .freeText(let spec): FreeTextControl(spec: spec, label: label, text: $draft.text)
        case .yesNo(let spec): YesNoControl(spec: spec, yes: $draft.yes)
        case .date(let spec):
            DateControl(spec: spec, isRequired: widget.required, date: $draft.date)
        case .path(let spec): PathControl(spec: spec, path: $draft.path)
        case .scale(let spec): ScaleControl(spec: spec, step: $draft.scale)
        case nil: EmptyView()
        }
    }
}

/// An option's label, with its detail beneath when it has one.
private struct ChoiceLabel: View {
    let choice: Hearth_V1_Choice

    var body: some View {
        VStack(alignment: .leading, spacing: Theme.Spacing.xs) {
            Text(choice.label)
                .font(Theme.body)
                .foregroundStyle(Theme.color(.textPrimary))
            if !choice.detail.isEmpty {
                Text(choice.detail)
                    .font(Theme.caption)
                    .foregroundStyle(Theme.color(.textSecondary))
            }
        }
        .fixedSize(horizontal: false, vertical: true)
    }
}

private struct SingleChoiceControl: View {
    /// A row of the radio group: an option, or Other.
    enum Row: Hashable {
        case option(String)
        case other
    }

    let spec: Hearth_V1_SingleChoice
    @Binding var draft: WidgetDraft

    var body: some View {
        VStack(alignment: .leading, spacing: Theme.Spacing.sm) {
            Picker("Choose one", selection: selection) {
                ForEach(spec.options, id: \.id) { option in
                    ChoiceLabel(choice: option).tag(Row?.some(.option(option.id)))
                }
                if spec.allowOther {
                    Text("Other…").font(Theme.body).tag(Row?.some(.other))
                }
            }
            .pickerStyle(.radioGroup)
            .labelsHidden()
            if draft.choosesOther {
                TextField("Other", text: $draft.other, prompt: Text("Say what you'd choose"))
                    .textFieldStyle(.roundedBorder)
                    .font(Theme.body)
                    .accessibilityLabel("Other answer")
            }
        }
    }

    private var selection: Binding<Row?> {
        Binding {
            draft.choosesOther ? .other : draft.choice.map(Row.option)
        } set: { row in
            draft.choosesOther = row == .other
            if case .option(let id) = row { draft.choice = id } else { draft.choice = nil }
        }
    }
}

private struct MultipleChoiceControl: View {
    let spec: Hearth_V1_MultipleChoice
    @Binding var draft: WidgetDraft

    var body: some View {
        VStack(alignment: .leading, spacing: Theme.Spacing.sm) {
            ForEach(spec.options, id: \.id) { option in
                Toggle(isOn: ticked(option.id)) { ChoiceLabel(choice: option) }
                    .toggleStyle(.checkbox)
            }
            if let caption = WidgetControl.caption(spec, chosen: draft.choices.count) {
                Text(caption)
                    .font(Theme.caption)
                    .foregroundStyle(Theme.color(isOver ? Tone.negative : Tone.neutral))
                    .monospacedDigit()
            }
        }
    }

    private var isOver: Bool {
        spec.max > 0 && draft.choices.count > Int(spec.max)
    }

    private func ticked(_ id: String) -> Binding<Bool> {
        Binding {
            draft.choices.contains(id)
        } set: { isOn in
            if isOn { draft.choices.insert(id) } else { draft.choices.remove(id) }
        }
    }
}

private struct FreeTextControl: View {
    let spec: Hearth_V1_FreeText
    let label: String
    @Binding var text: String

    @ScaledMetric(relativeTo: .body) private var editorHeight: CGFloat = Theme.Size.avatarLarge * 2

    var body: some View {
        VStack(alignment: .trailing, spacing: Theme.Spacing.xs) {
            if spec.multiline {
                TextEditor(text: $text)
                    .font(Theme.body)
                    .scrollContentBackground(.hidden)
                    .frame(minHeight: editorHeight)
                    .overlay(alignment: .topLeading) { placeholder }
                    .cardSurface()
                    .accessibilityLabel(label)
            } else {
                TextField(label, text: $text, prompt: Text(prompt))
                    .textFieldStyle(.roundedBorder)
                    .font(Theme.body)
            }
            if let count = WidgetControl.count(text, spec) {
                Text(count)
                    .font(Theme.caption)
                    .foregroundStyle(
                        Theme.color(text.count > WidgetDraft.limit(spec) ? Tone.negative : .neutral)
                    )
                    .monospacedDigit()
                    .accessibilityLabel("\(text.count) of \(WidgetDraft.limit(spec)) characters")
            }
        }
    }

    private var prompt: String {
        spec.placeholder.isEmpty ? "Type your answer" : spec.placeholder
    }

    @ViewBuilder
    private var placeholder: some View {
        if text.isEmpty {
            Text(prompt)
                .font(Theme.body)
                .foregroundStyle(Theme.color(.textSecondary))
                .padding(.leading, Theme.Spacing.xs)
                .allowsHitTesting(false)
                .accessibilityHidden(true)
        }
    }
}

private struct YesNoControl: View {
    let spec: Hearth_V1_YesNo
    @Binding var yes: Bool?

    var body: some View {
        HStack(spacing: Theme.Spacing.sm) {
            button(WidgetDraft.yesLabel(spec), value: true)
            button(WidgetDraft.noLabel(spec), value: false)
        }
    }

    /// A bordered toggle button: the system draws the chosen one tinted and
    /// VoiceOver says which is on.
    private func button(_ label: String, value: Bool) -> some View {
        Toggle(
            label,
            isOn: Binding {
                yes == value
            } set: {
                if $0 { yes = value }
            }
        )
        .toggleStyle(.button)
        .buttonStyle(.bordered)
    }
}

private struct DateControl: View {
    let spec: Hearth_V1_DatePick
    let isRequired: Bool
    @Binding var date: Date?

    var body: some View {
        VStack(alignment: .leading, spacing: Theme.Spacing.xs) {
            if spec.withTime {
                DatePicker(
                    "Date and time", selection: selection, in: range,
                    displayedComponents: [.date, .hourAndMinute]
                )
                .datePickerStyle(.field)
            } else {
                DatePicker("Date", selection: selection, in: range, displayedComponents: .date)
                    .datePickerStyle(.graphical)
                    .labelsHidden()
            }
            if date == nil {
                Text("No date chosen yet")
                    .font(Theme.caption)
                    .foregroundStyle(Theme.color(.textSecondary))
            } else if let date, !WidgetDraft.isInRange(date, spec) {
                Text("Choose a date in the range shown")
                    .font(Theme.caption)
                    .foregroundStyle(Theme.color(Tone.negative))
            }
        }
        // Clicking the day a picker already shows changes nothing, so a
        // required date starts on that day rather than on none.
        .onAppear { if isRequired && date == nil { date = fallback } }
    }

    /// The picker shows the owner's date, else the fallback day.
    private var selection: Binding<Date> {
        Binding {
            date ?? fallback
        } set: {
            date = $0
        }
    }

    /// Today, or the nearest day the range allows.
    private var fallback: Date {
        max(range.lowerBound, min(.now, range.upperBound))
    }

    private var range: ClosedRange<Date> {
        let earliest =
            spec.hasEarliest ? WidgetDraft.localDay(spec.earliest.date) : Date.distantPast
        let latest =
            spec.hasLatest
            ? Calendar.current.date(
                byAdding: DateComponents(day: 1, second: -1),
                to: WidgetDraft.localDay(spec.latest.date)) ?? Date.distantFuture
            : Date.distantFuture
        return earliest...max(earliest, latest)
    }
}

private struct PathControl: View {
    let spec: Hearth_V1_PathPick
    @Binding var path: String

    @Environment(\.pickFolder) private var pickFolder

    var body: some View {
        HStack(spacing: Theme.Spacing.sm) {
            icon
            Text(path.isEmpty ? "Nothing chosen" : path)
                .font(Theme.body)
                .foregroundStyle(Theme.color(path.isEmpty ? .textSecondary : .textPrimary))
                .lineLimit(1)
                .truncationMode(.middle)
                .help(path)
            Spacer(minLength: Theme.Spacing.sm)
            Button("Choose…") {
                Task {
                    let start = path.isEmpty ? spec.startPath : path
                    let chosen = await pickFolder.pickPath(
                        folders: spec.folders, files: spec.files, start: start)
                    if let chosen { path = chosen }
                }
            }
            .accessibilityHint(hint)
        }
        .accessibilityElement(children: .contain)
    }

    @ViewBuilder
    private var icon: some View {
        if isFolder {
            Image(systemName: "folder")
                .font(Theme.body)
                .foregroundStyle(Theme.color(.textSecondary))
                .accessibilityHidden(true)
        } else {
            FileIcon(filename: (path as NSString).lastPathComponent)
        }
    }

    /// Whether the row shows a folder: nothing chosen yet, a question that
    /// takes folders only, or a chosen path that is a folder on this Mac.
    private var isFolder: Bool {
        if path.isEmpty || !spec.files { return true }
        if !spec.folders { return false }
        var isDirectory: ObjCBool = false
        return FileManager.default.fileExists(atPath: path, isDirectory: &isDirectory)
            && isDirectory.boolValue
    }

    private var hint: String {
        switch (spec.folders, spec.files) {
        case (true, true): "Chooses a folder or a file"
        case (false, true): "Chooses a file"
        default: "Chooses a folder"
        }
    }
}

private struct ScaleControl: View {
    let spec: Hearth_V1_Scale
    @Binding var step: Int32?

    var body: some View {
        VStack(spacing: Theme.Spacing.xs) {
            Picker("Rating from \(spec.min) to \(spec.max)", selection: $step) {
                ForEach(steps, id: \.self) { value in
                    Text("\(value)").tag(Int32?.some(value))
                }
            }
            .pickerStyle(.segmented)
            .labelsHidden()
            HStack {
                Text(spec.minLabel)
                Spacer(minLength: Theme.Spacing.sm)
                Text(spec.maxLabel)
            }
            .font(Theme.caption)
            .foregroundStyle(Theme.color(.textSecondary))
            .accessibilityHidden(spec.minLabel.isEmpty && spec.maxLabel.isEmpty)
        }
        // The end labels sit under the ends of the segments.
        .fixedSize()
    }

    /// The steps, at most the catalogue's ten.
    private var steps: [Int32] {
        spec.min <= spec.max ? Array((spec.min...spec.max).prefix(10)) : []
    }
}
