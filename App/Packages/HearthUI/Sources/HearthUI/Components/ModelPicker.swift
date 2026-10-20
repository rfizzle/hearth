// ModelPicker chooses one model id from the list a service served: a bordered
// button showing the choice that opens a popover with a search field over the
// matching ids. It is a system button, popover, text field and list, so the
// keyboard and VoiceOver behave as they do everywhere. It only picks; the
// service card falls back to a typed id when the list could not be read.
// See docs/design.md#forms-sheets-and-settings

import HearthDesign
import SwiftUI

/// A searchable choice among the model ids a service lists.
struct ModelPicker: View {
    let models: [String]
    @Binding var selection: String

    @State private var isOpen = false
    @State private var query = ""

    private var matches: [String] {
        let text = query.trimmingCharacters(in: .whitespaces)
        guard !text.isEmpty else { return models }
        return models.filter { $0.localizedCaseInsensitiveContains(text) }
    }

    var body: some View {
        Button {
            isOpen = true
        } label: {
            HStack {
                Text(selection.isEmpty ? "Choose a model" : selection)
                    .lineLimit(1)
                    .truncationMode(.middle)
                Spacer(minLength: Theme.Spacing.sm)
                Image(systemName: "chevron.up.chevron.down")
                    .font(Theme.caption)
                    .foregroundStyle(Theme.color(.textSecondary))
                    .accessibilityHidden(true)
            }
            .frame(maxWidth: .infinity)
        }
        .buttonStyle(.bordered)
        .accessibilityLabel("Model")
        .accessibilityValue(selection)
        .popover(isPresented: $isOpen, arrowEdge: .bottom) {
            VStack(alignment: .leading, spacing: Theme.Spacing.sm) {
                TextField("Search models", text: $query)
                    .textFieldStyle(.roundedBorder)
                    .accessibilityLabel("Search models")
                if matches.isEmpty {
                    Text("No model matches.")
                        .font(Theme.callout)
                        .foregroundStyle(Theme.color(.textSecondary))
                } else {
                    List(matches, id: \.self) { model in
                        Button {
                            selection = model
                            isOpen = false
                        } label: {
                            HStack {
                                Text(model)
                                Spacer()
                                if model == selection {
                                    Image(systemName: "checkmark")
                                        .accessibilityLabel("Chosen")
                                }
                            }
                            .contentShape(Rectangle())
                        }
                        .buttonStyle(.plain)
                    }
                    .listStyle(.plain)
                }
            }
            .padding(Theme.Spacing.md)
            .frame(width: 280, height: 260)
        }
    }
}
