// ServicePicker is the card for a service that has no card yet: at rest a
// mark, a name and one button; opened, the services hearth knows as rows of a
// mark and a name, searchable when there are more than eight, a runtime found
// running on this Mac first with a "Running" pill and how many models it has,
// and "Custom gateway" last; a service chosen, its fields inside the same card, prefilled
// and editable, with Save and Cancel and the core's refusal under them. The
// rows are system buttons and the fields system text fields, so keyboard and
// VoiceOver behaviour is theirs; the card is one labelled group. Like the
// provider cards it has no hover fill: it holds controls and is not a choice.
// The core decides whether what was typed is a service it can use; Save sends
// it as it is. Each stage is a layout move.
// See docs/design.md#forms-sheets-and-settings and docs/design.md#motion

import HearthDesign
import HearthKit
import SwiftUI

/// The card that adds a service by choosing it from a list or by address.
public struct ServicePicker: View {
    /// How far the card is open.
    public enum Stage: Equatable, Sendable {
        /// The known services as rows.
        case list
        /// The fields, prefilled from a known service, or empty for a custom
        /// gateway when nil.
        case fields(Hearth_V1_KnownService?)
    }

    let title: String
    let services: [Hearth_V1_KnownService]
    let failure: String?
    @Binding var stage: Stage?
    let onChooseBuiltin: (Hearth_V1_KnownService) -> Void
    let onSave: (Hearth_V1_SetAPIKeyRequest) -> Void
    let onListModels: ModelLister?

    @State private var query = ""
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    /// A picker titled `title` over `services`. A builtin row is handed to
    /// `onChooseBuiltin`, since its own card signs it in; any other row, or
    /// the custom gateway, opens the fields. `onListModels`, when given, reads
    /// the models a typed address and key serve, nil when it could not.
    public init(
        title: String,
        services: [Hearth_V1_KnownService],
        failure: String?,
        stage: Binding<Stage?>,
        onChooseBuiltin: @escaping (Hearth_V1_KnownService) -> Void,
        onSave: @escaping (Hearth_V1_SetAPIKeyRequest) -> Void,
        onListModels: ModelLister? = nil
    ) {
        self.title = title
        self.services = services
        self.failure = failure
        self._stage = stage
        self.onChooseBuiltin = onChooseBuiltin
        self.onSave = onSave
        self.onListModels = onListModels
    }

    nonisolated static let searchThreshold = 8
    static let customName = "Custom gateway"
    static let gives =
        "Anthropic, Gemini, Z.ai, Groq, a company gateway, or any service that speaks OpenAI's API."

    public var body: some View {
        CardSurface {
            HStack(alignment: .top, spacing: Theme.Spacing.md) {
                Avatar(.symbol("plus"), size: .large)
                VStack(alignment: .leading, spacing: Theme.Spacing.xs) {
                    Text(title)
                        .font(Theme.headline)
                        .foregroundStyle(Theme.color(.textPrimary))
                    Text(Self.gives)
                        .font(Theme.callout)
                        .foregroundStyle(Theme.color(.textSecondary))
                    content
                        .padding(.top, Theme.Spacing.xs)
                }
                // The card spans the column like the provider cards above it.
                .frame(maxWidth: .infinity, alignment: .leading)
            }
        }
        .accessibilityElement(children: .contain)
        .accessibilityLabel(title)
        .accessibilityHint(Self.gives)
    }

    @ViewBuilder
    private var content: some View {
        switch stage {
        case nil:
            Button("Choose a Service") { setStage(.list) }
        case .list:
            list
        case .fields(let service):
            ServiceFields(
                service: service, failure: failure,
                onCancel: { setStage(nil) }, onSave: onSave, onListModels: onListModels
            )
            .id(service?.id ?? Self.customName)
        }
    }

    private var list: some View {
        VStack(alignment: .leading, spacing: Theme.Spacing.xs) {
            if services.count > Self.searchThreshold {
                TextField("Search services", text: $query)
                    .textFieldStyle(.roundedBorder)
                    .accessibilityLabel("Search services")
            }
            ForEach(matches, id: \.id) { service in
                row(service.name, detail: Self.detail(service)) {
                    if service.kind == .builtin {
                        setStage(nil)
                        onChooseBuiltin(service)
                    } else {
                        setStage(.fields(service))
                    }
                }
            }
            row(Self.customName) { setStage(.fields(nil)) }
            HStack {
                Spacer()
                Button("Cancel") { setStage(nil) }
                    .keyboardShortcut(.cancelAction)
            }
        }
        .transition(.opacity)
    }

    private var matches: [Hearth_V1_KnownService] {
        let text = query.trimmingCharacters(in: .whitespaces)
        let ordered = Self.ordered(services)
        guard !text.isEmpty else { return ordered }
        return ordered.filter { $0.name.localizedCaseInsensitiveContains(text) }
    }

    /// `services` with a runtime found running on this Mac first, since it
    /// costs nothing to use; the core's order holds within each group.
    nonisolated static func ordered(_ all: [Hearth_V1_KnownService]) -> [Hearth_V1_KnownService] {
        all.filter(\.detected) + all.filter { !$0.detected }
    }

    /// The caption under a running runtime's name: how many models it has.
    nonisolated static func detail(_ service: Hearth_V1_KnownService) -> String? {
        guard service.detected else { return nil }
        let count = service.models.count
        return count == 1 ? "1 model" : "\(count) models"
    }

    private func row(
        _ name: String, detail: String? = nil, action: @escaping () -> Void
    ) -> some View {
        Button(action: action) {
            HStack(spacing: Theme.Spacing.sm) {
                ServiceMark(named: name, size: .regular)
                VStack(alignment: .leading, spacing: 0) {
                    Text(name)
                        .font(Theme.body)
                        .foregroundStyle(Theme.color(.textPrimary))
                    if let detail {
                        Text(detail)
                            .font(Theme.caption)
                            .foregroundStyle(Theme.color(.textSecondary))
                    }
                }
                Spacer()
                if detail != nil { Pill("Running", tone: .positive) }
                Image(systemName: "chevron.right")
                    .foregroundStyle(Theme.color(.textSecondary))
                    .accessibilityHidden(true)
            }
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
        .accessibilityLabel(detail.map { "\(name), running, \($0)" } ?? name)
    }

    private func setStage(_ next: Stage?) {
        withAnimation(Theme.motion(Theme.move, reduceMotion: reduceMotion)) {
            stage = next
        }
        if next == nil { query = "" }
    }
}

/// Reads the models a service serves from its address and key; nil when it
/// could not. An empty key asks for a saved service at that address by the key
/// the core holds.
public typealias ModelLister = @Sendable (String, String) async -> [String]?

extension ServicePicker {
    /// The states the gallery and previews show.
    public enum PreviewState: String, CaseIterable, Sendable {
        case closed
        case list
        case knownService
        case customGateway
        case failed

        static let failureText = "use an https address; over http the key could be read on the way"

        var stage: Stage? {
            switch self {
            case .closed: nil
            case .list: .list
            case .knownService: .fields(Fixtures.knownServices[1])
            case .customGateway, .failed: .fields(nil)
            }
        }
    }

    /// A picker showing `state`, for the gallery and previews.
    public init(previewState state: PreviewState) {
        self.init(
            title: "Another service", services: Fixtures.knownServices,
            failure: state == .failed ? PreviewState.failureText : nil,
            stage: .constant(state.stage), onChooseBuiltin: { _ in }, onSave: { _ in },
            onListModels: { _, _ in Fixtures.ollamaModels })
    }
}

#Preview("Closed") { ServicePicker(previewState: .closed).padding() }
#Preview("List") { ServicePicker(previewState: .list).padding() }
#Preview("Known service") { ServicePicker(previewState: .knownService).padding() }
#Preview("Custom gateway") { ServicePicker(previewState: .customGateway).padding() }
#Preview("Failed") { ServicePicker(previewState: .failed).padding() }
