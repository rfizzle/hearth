// BudgetField is the small form in the popover behind "Change…": one amount
// in dollars a month, a line saying what the limit does, Cancel and Save. A
// blank amount removes the limit. It is the only field in the usage pieces,
// and it hands back whole cents, which is what the core stores; what a limit
// does when it is reached is the core's to do.
// See docs/design.md#forms-sheets-and-settings

import HearthDesign
import SwiftUI

/// The amount a bot or the whole team may spend in a month.
public struct BudgetField: View {
    let subject: String
    let onSave: (Int64) -> Void
    let onCancel: () -> Void

    @State private var text: String
    @FocusState private var focused: Bool

    /// A field for `subject` ("Dana", "the team") currently limited to
    /// `cents` (0 is no limit). `onSave` gets the new cents.
    public init(
        subject: String, cents: Int64, onSave: @escaping (Int64) -> Void,
        onCancel: @escaping () -> Void
    ) {
        self.subject = subject
        self.onSave = onSave
        self.onCancel = onCancel
        _text = State(initialValue: Self.text(cents: cents))
    }

    /// The cents the text means: 0 for blank, nil for something that is not
    /// an amount of dollars.
    static func cents(from text: String) -> Int64? {
        let trimmed = text.replacing("$", with: "").trimmingCharacters(in: .whitespaces)
        if trimmed.isEmpty { return 0 }
        guard let dollars = try? Decimal(trimmed, format: .number), dollars >= 0 else {
            return nil
        }
        return NSDecimalNumber(decimal: dollars * 100).rounding(accordingToBehavior: nil)
            .int64Value
    }

    /// The amount as the field starts: whole dollars, or with cents; blank
    /// for none.
    static func text(cents: Int64) -> String {
        guard cents > 0 else { return "" }
        let digits = cents % 100 == 0 ? 0 : 2
        return (Double(cents) / 100)
            .formatted(.number.precision(.fractionLength(digits)).grouping(.never))
    }

    private var parsed: Int64? { Self.cents(from: text) }

    private var explanation: String {
        "\(subject) pauses when the month's cost reaches this. Leave it blank for no limit."
    }

    public var body: some View {
        VStack(alignment: .leading, spacing: Theme.Spacing.md) {
            Text("Monthly limit")
                .font(Theme.headline)
            HStack(spacing: Theme.Spacing.sm) {
                TextField("Amount", text: $text, prompt: Text("No limit"))
                    .textFieldStyle(.roundedBorder)
                    .multilineTextAlignment(.trailing)
                    .monospacedDigit()
                    .frame(width: 96)
                    .focused($focused)
                    .accessibilityLabel("Monthly limit in dollars")
                Text("US dollars a month")
                    .font(Theme.callout)
                    .foregroundStyle(Theme.color(.textSecondary))
            }
            Text(explanation)
                .font(Theme.caption)
                .foregroundStyle(Theme.color(.textSecondary))
                .fixedSize(horizontal: false, vertical: true)
            HStack {
                Spacer()
                Button("Cancel", action: onCancel)
                    .keyboardShortcut(.cancelAction)
                Button("Save", action: save)
                    .buttonStyle(.borderedProminent)
                    .keyboardShortcut(.defaultAction)
                    .disabled(parsed == nil)
            }
        }
        .padding(Theme.Spacing.lg)
        .frame(width: 280)
        .onSubmit(save)
        .task { focused = true }
    }

    private func save() {
        if let parsed { onSave(parsed) }
    }
}

extension BudgetField {
    /// The states the gallery and previews show.
    public enum PreviewState: String, CaseIterable, Sendable {
        case limited
        case none

        var cents: Int64 { self == .limited ? 2000 : 0 }
    }

    /// A field showing `state`, for the gallery and previews.
    public init(previewState state: PreviewState) {
        self.init(subject: "Dana", cents: state.cents, onSave: { _ in }, onCancel: {})
    }
}

#Preview("Limited") { BudgetField(previewState: .limited) }
#Preview("None") { BudgetField(previewState: .none) }
