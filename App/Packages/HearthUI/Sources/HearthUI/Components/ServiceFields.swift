// ServiceFields are the fields of a service being added or changed, inside the
// card that holds them: a name, an address, a key and the model, with Save and
// Cancel and the core's refusal under them. The model is chosen from the list
// the service serves, through ModelPicker, once it can be read, and typed
// before that. They are system text fields, so keyboard and VoiceOver
// behaviour is theirs. The core decides whether what was typed is a service it
// can use; Save sends it as it is.
// See docs/design.md#forms-sheets-and-settings

import HearthDesign
import HearthKit
import SwiftUI

/// The fields of a service being added or changed: its name and address
/// prefilled from the known service, or from the saved account when
/// `account` is given, a key, and the default model with a hint. A changed
/// service's key field is empty and says an empty key keeps the one stored.
struct ServiceFields: View {
    let service: Hearth_V1_KnownService?
    let account: Hearth_V1_ProviderAccount?
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
        service: Hearth_V1_KnownService?, account: Hearth_V1_ProviderAccount? = nil,
        failure: String?,
        onCancel: @escaping () -> Void,
        onSave: @escaping (Hearth_V1_SetAPIKeyRequest) -> Void,
        onListModels: ModelLister? = nil
    ) {
        self.service = service
        self.account = account
        self.failure = failure
        self.onCancel = onCancel
        self.onSave = onSave
        self.onListModels = onListModels
        _name = State(initialValue: account?.name ?? service?.name ?? "")
        _address = State(initialValue: account?.baseURL ?? service?.baseURL ?? "")
        let seed = Self.seed(service)
        let preferred = account?.defaultModel ?? service?.defaultModel ?? ""
        _model = State(
            initialValue: seed.map { $0.models.contains(preferred) ? preferred : $0.models[0] }
                ?? preferred)
        _models = State(initialValue: account == nil ? seed?.models : nil)
    }

    /// The address and models a runtime found running just now listed, so the
    /// picker opens with them before the card asks again.
    private static func seed(_ service: Hearth_V1_KnownService?) -> (
        address: String, models: [String]
    )? {
        guard let service, service.detected, !service.models.isEmpty else { return nil }
        return (service.baseURL, service.models)
    }

    /// A runtime on this Mac takes no key; neither does a saved service that
    /// holds none.
    private var isLocal: Bool {
        if let account { return account.label.isEmpty }
        return service?.kind == .local
    }

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
                    prompt: Text(keyPrompt)
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
        .task {
            focus = account != nil || service == nil ? .name : (isLocal ? .model : .key)
        }
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
        let address = address.trimmingCharacters(in: .whitespaces)
        if let seed = Self.seed(service), account == nil, address == seed.address, key.isEmpty {
            models = seed.models
            isListing = false
            return
        }
        models = nil
        isListing = false
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

    private var keyPrompt: String {
        if account != nil && !isLocal { return "Leave empty to keep the key" }
        return isLocal ? "Not needed on this Mac" : "Paste a key"
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
            Self.request(
                profile: account?.profile ?? "", name: name, address: address, key: key,
                model: model))
    }

    /// What Save sends. An empty key is sent as it is: for a saved service the
    /// core keeps the stored key, and refuses an address on another host.
    nonisolated static func request(
        profile: String, name: String, address: String, key: String, model: String
    ) -> Hearth_V1_SetAPIKeyRequest {
        .with {
            $0.provider = .openaiCompatible
            $0.profile = profile
            $0.displayName = name
            $0.baseURL = address
            $0.key = key
            $0.defaultModel = model
        }
    }
}
