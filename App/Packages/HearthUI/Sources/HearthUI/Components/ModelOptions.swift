// ModelOptions is what the tier rows and the Advanced model choice share: the
// words for one model the owner may pick, the string a bot stores for it, and
// the options grouped under their provider's label. TierRow is a tier's name,
// what it means and a menu to choose its model; ModelOptionPicker is the
// searchable choice for one bot, with a row for "no pin". Both are system
// controls (a menu Picker, ModelPicker's popover), so the keyboard and
// VoiceOver behave as they do everywhere, and both only edit a binding.
// See docs/design.md#forms-sheets-and-settings

import HearthDesign
import HearthKit
import SwiftUI

extension Hearth_V1_ModelOption {
    /// The model in words: "Anthropic · claude-opus-4-5".
    public var title: String { label.isEmpty ? model : "\(label) · \(model)" }

    /// The string a bot stores to pin this model: the provider, the added
    /// service's id and the model, joined by colons.
    public var pinned: String { "\(provider.wireName):\(profile):\(model)" }

    /// Whether `choice` names this model.
    public func matches(_ choice: Hearth_V1_TierChoice) -> Bool {
        provider == choice.provider && profile == choice.profile && model == choice.model
    }

    /// The option a stored pin names, with `label` as the words for it; nil
    /// for anything that is not a pin.
    public init?(pinned: String, label: String = "") {
        let parts = pinned.split(separator: ":", maxSplits: 2, omittingEmptySubsequences: false)
        let known: [Hearth_V1_Provider] = [
            .chatgpt, .anthropic, .gemini, .openrouter, .openaiCompatible,
        ]
        guard parts.count == 3, !parts[2].isEmpty,
            let provider = known.first(where: { $0.wireName == parts[0] })
        else { return nil }
        self.init()
        self.provider = provider
        self.profile = String(parts[1])
        self.model = String(parts[2])
        self.label = label
    }

    /// `options` under their labels, in the order the labels first appear.
    public static func grouped(_ options: [Hearth_V1_ModelOption]) -> [(
        label: String, options: [Hearth_V1_ModelOption]
    )] {
        var groups: [(label: String, options: [Hearth_V1_ModelOption])] = []
        for option in options {
            if let index = groups.firstIndex(where: { $0.label == option.label }) {
                groups[index].options.append(option)
            } else {
                groups.append((option.label, [option]))
            }
        }
        return groups
    }
}

extension Hearth_V1_Provider {
    /// The name the core stores a provider under in a pinned model.
    fileprivate var wireName: String {
        switch self {
        case .chatgpt: "chatgpt"
        case .anthropic: "anthropic"
        case .gemini: "gemini"
        case .openrouter: "openrouter"
        case .openaiCompatible: "openai-compatible"
        default: ""
        }
    }
}

/// One tier: its name, what it means and a menu of the models it may use.
public struct TierRow: View {
    let tier: Hearth_V1_Tier
    let options: [Hearth_V1_ModelOption]
    @Binding var selection: Hearth_V1_ModelOption

    public init(
        tier: Hearth_V1_Tier, options: [Hearth_V1_ModelOption],
        selection: Binding<Hearth_V1_ModelOption>
    ) {
        self.tier = tier
        self.options = options
        self._selection = selection
    }

    public var body: some View {
        HStack(alignment: .firstTextBaseline, spacing: Theme.Spacing.md) {
            VStack(alignment: .leading, spacing: Theme.Spacing.xs) {
                Text(TierPicker.name(tier))
                    .font(Theme.body)
                    .foregroundStyle(Theme.color(.textPrimary))
                Text(TierPicker.hint(tier))
                    .font(Theme.caption)
                    .foregroundStyle(Theme.color(.textSecondary))
            }
            Spacer(minLength: Theme.Spacing.md)
            Picker(TierPicker.name(tier), selection: $selection) {
                ForEach(Hearth_V1_ModelOption.grouped(shown), id: \.label) { group in
                    Section(group.label) {
                        ForEach(group.options, id: \.pinned) { option in
                            Text(option.model).tag(option)
                        }
                    }
                }
            }
            .pickerStyle(.menu)
            .labelsHidden()
            .fixedSize()
            .accessibilityLabel("\(TierPicker.name(tier)) model")
        }
        .padding(.vertical, Theme.Spacing.sm)
    }

    /// The options, with the current choice among them even when the
    /// accounts no longer offer it, so the menu never shows a blank.
    private var shown: [Hearth_V1_ModelOption] {
        options.contains(selection) ? options : options + [selection]
    }
}

/// The searchable choice of a model to pin for one bot, with a first row that
/// leaves the pin off so the tier decides.
public struct ModelOptionPicker: View {
    let options: [Hearth_V1_ModelOption]
    @Binding var pinned: String

    public static let tierTitle = "Use the tier's model"

    public init(options: [Hearth_V1_ModelOption], pinned: Binding<String>) {
        self.options = options
        self._pinned = pinned
    }

    public var body: some View {
        ModelPicker(
            models: [Self.tierTitle] + options.map(\.title),
            selection: Binding(
                get: { options.first { $0.pinned == pinned }?.title ?? Self.tierTitle },
                set: { title in
                    pinned = options.first { $0.title == title }?.pinned ?? ""
                }))
    }
}
