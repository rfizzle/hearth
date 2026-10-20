// ServicePicker is the card for a service that has no card yet: at rest a
// mark, a name and one button; opened, the services hearth knows as rows of a
// mark and a name, searchable when there are more than eight, with "Custom
// gateway" last; a service chosen, its fields inside the same card, prefilled
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
                row(service.name) {
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
        guard !text.isEmpty else { return services }
        return services.filter { $0.name.localizedCaseInsensitiveContains(text) }
    }

    private func row(_ name: String, action: @escaping () -> Void) -> some View {
        Button(action: action) {
            HStack(spacing: Theme.Spacing.sm) {
                ServiceMark(named: name, size: .regular)
                Text(name)
                    .font(Theme.body)
                    .foregroundStyle(Theme.color(.textPrimary))
                Spacer()
                Image(systemName: "chevron.right")
                    .foregroundStyle(Theme.color(.textSecondary))
                    .accessibilityHidden(true)
            }
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
        .accessibilityLabel(name)
    }

    private func setStage(_ next: Stage?) {
        withAnimation(Theme.motion(Theme.move, reduceMotion: reduceMotion)) {
            stage = next
        }
        if next == nil { query = "" }
    }
}

/// The fields of a service being added: its name and address prefilled from
/// the known service and editable, a key, and the default model with a hint.
struct ServiceFields: View {
    let service: Hearth_V1_KnownService?
    let failure: String?
    let onCancel: () -> Void
    let onSave: (Hearth_V1_SetAPIKeyRequest) -> Void
    let onListModels: ModelLister?

    private enum Field { case name, address, key, model }

    @State private var name: String
    @State private var address: String
    @State private var key = ""
    @State private var model: String
    /// The ids the service listed for the address and key typed; nil until
    /// they are read, and again when they could not be.
    @State private var models: [String]?
    @State private var isListing = false
    @FocusState private var focus: Field?

    init(
        service: Hearth_V1_KnownService?, failure: String?,
        onCancel: @escaping () -> Void,
        onSave: @escaping (Hearth_V1_SetAPIKeyRequest) -> Void,
        onListModels: ModelLister? = nil
    ) {
        self.service = service
        self.failure = failure
        self.onCancel = onCancel
        self.onSave = onSave
        self.onListModels = onListModels
        _name = State(initialValue: service?.name ?? "")
        _address = State(initialValue: service?.baseURL ?? "")
        _model = State(initialValue: service?.defaultModel ?? "")
    }

    private var isLocal: Bool { service?.kind == .local }

    var body: some View {
        VStack(alignment: .leading, spacing: Theme.Spacing.sm) {
            labelled("Name") {
                TextField("Name", text: $name, prompt: Text("My company's gateway"))
                    .focused($focus, equals: .name)
            }
            labelled("Address") {
                TextField("Address", text: $address, prompt: Text("https://example.com/v1"))
                    .focused($focus, equals: .address)
            }
            labelled("Key") {
                SecureField(
                    "Key", text: $key,
                    prompt: Text(isLocal ? "Not needed on this Mac" : "Paste a key")
                )
                .focused($focus, equals: .key)
            }
            labelled("Model") {
                HStack(spacing: Theme.Spacing.sm) {
                    if let models {
                        ModelPicker(models: models, selection: $model)
                    } else {
                        TextField("Model", text: $model, prompt: Text("gpt-4o"))
                            .focused($focus, equals: .model)
                    }
                    if isListing {
                        ProgressView()
                            .controlSize(.small)
                            .accessibilityLabel("Reading the service's models")
                    }
                }
            }
            Text(
                models == nil
                    ? "The model your team uses, as the service names it."
                    : "The models this service offers."
            )
            .font(Theme.caption)
            .foregroundStyle(Theme.color(.textSecondary))
            if let service, service.kind == .compatible, let help = URL(string: service.helpURL) {
                Link("Where do I get a key?", destination: help)
                    .font(Theme.callout)
            }
            if let failure { FailureLabel(text: failure) }
            HStack {
                Spacer()
                Button("Cancel", action: onCancel)
                    .keyboardShortcut(.cancelAction)
                Button("Save", action: save)
                    .buttonStyle(.borderedProminent)
                    .keyboardShortcut(.defaultAction)
                    .disabled(isIncomplete)
            }
        }
        .textFieldStyle(.roundedBorder)
        .onSubmit(save)
        .task { focus = service == nil ? .name : (isLocal ? .model : .key) }
        .task(id: Lookup(address: address, key: key)) { await listModels() }
        .transition(.opacity)
    }

    /// What the model list depends on.
    private struct Lookup: Equatable {
        let address: String
        let key: String
    }

    /// Reads the service's models once the address and key are typed and the
    /// typing has paused; a list that could not be read leaves the typed id.
    private func listModels() async {
        models = nil
        isListing = false
        let address = address.trimmingCharacters(in: .whitespaces)
        guard let onListModels, !address.isEmpty, !key.isEmpty || isLocal else { return }
        do { try await Task.sleep(for: .milliseconds(600)) } catch { return }
        isListing = true
        let listed = await onListModels(address, key)
        guard !Task.isCancelled else { return }
        isListing = false
        models = listed?.isEmpty == false ? listed : nil
    }

    /// Save waits for what every service needs; whether a key is needed is
    /// the core's to say.
    private var isIncomplete: Bool {
        [name, address, model].contains { $0.trimmingCharacters(in: .whitespaces).isEmpty }
    }

    private func labelled<Content: View>(
        _ label: String, @ViewBuilder _ content: () -> Content
    ) -> some View {
        VStack(alignment: .leading, spacing: Theme.Spacing.xs) {
            Text(label)
                .font(Theme.caption)
                .foregroundStyle(Theme.color(.textSecondary))
                .accessibilityHidden(true)
            content()
                .accessibilityLabel(label)
        }
    }

    private func save() {
        guard !isIncomplete else { return }
        onSave(
            .with {
                $0.provider = .openaiCompatible
                $0.displayName = name
                $0.baseURL = address
                $0.key = key
                $0.defaultModel = model
            })
    }
}

/// Reads the models a service serves from its address and key; nil when it
/// could not.
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
