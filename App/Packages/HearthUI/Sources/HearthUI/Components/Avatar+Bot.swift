// The avatars the transcript and the sidebar draw for a bot and for the
// owner. A bot's avatar text that is plain ASCII is an SF Symbol name, any
// other text is a character such as an emoji, and an empty one shows the
// name's initials.

import HearthDesign
import HearthKit
import SwiftUI

extension Avatar {
    /// `bot`'s avatar.
    public init(bot: Hearth_V1_Bot, size: Size = .regular) {
        self.init(Self.content(name: bot.name, avatar: bot.avatar), size: size)
    }

    /// The owner's avatar.
    public static var owner: Avatar {
        Avatar(.symbol("person.fill"))
    }

    static func content(name: String, avatar: String) -> Content {
        if avatar.isEmpty {
            .initials(name)
        } else if avatar.allSatisfy(\.isASCII) {
            .symbol(avatar)
        } else {
            .character(avatar)
        }
    }
}
