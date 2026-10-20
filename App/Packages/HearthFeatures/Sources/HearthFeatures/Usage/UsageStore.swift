// UsageStore holds what the bots cost as the core last reported it: a summary
// for each month asked about, the months that have any usage, and for the
// current month each bot's budget and whether it is paused. It fetches a
// month, applies each usage tick the relay delivers (the bot's figures and the
// team's replace the ones shown, in the month the tick names), refetches on a
// resync, and turns Change… and Resume into calls whose answer is the summary
// to show. The core owns the pricing, the budgets and the pause; a bot is
// paused when the summary says so, which is where the sidebar and the
// composer read it. The inspector, the Settings pane and the Inbox's budget
// card share the one store.
// See docs/model.md#usage

import Foundation
import HearthKit
import Observation

/// The store behind the Usage section and pane.
@MainActor
@Observable
public final class UsageStore {
    /// The months with any usage, newest first.
    public private(set) var months: [String] = []
    /// The month the core calls this one, once the first summary is in.
    public private(set) var currentMonth: String?
    /// The month the Settings pane shows.
    public var selectedMonth: String?
    /// The last failed call's text, if one failed.
    public private(set) var error: String?

    private var summaries: [String: Hearth_V1_SummaryResponse] = [:]
    private let client: any HearthClient
    @ObservationIgnored private let relay: EventRelay
    @ObservationIgnored private var registration: EventRelay.Registration?

    /// A store over `client`, fed by `relay`.
    public init(client: any HearthClient, relay: EventRelay) {
        self.client = client
        self.relay = relay
        registration = relay.register { [weak self] event in await self?.handle(event) }
    }

    isolated deinit {
        if let registration { relay.unregister(registration) }
    }

    /// The summary fetched for `month`; the current month when empty.
    public func summary(month: String = "") -> Hearth_V1_SummaryResponse? {
        summaries[month.isEmpty ? (currentMonth ?? "") : month]
    }

    /// This month's summary.
    public var current: Hearth_V1_SummaryResponse? { summary() }

    /// The usage of `botID` this month, if the core has any to report.
    public func usage(of botID: String) -> Hearth_V1_BotUsage? {
        current?.bots.first { $0.botID == botID }
    }

    /// Whether `botID` is paused this month, by its own budget or the team's.
    /// Nothing is paused until a summary has come back.
    public func isPaused(_ botID: String) -> Bool {
        guard let current else { return false }
        return current.orgPaused || (usage(of: botID)?.paused ?? false)
    }

    /// The month as the owner reads it: "October 2026".
    public static func title(month: String, locale: Locale = .current) -> String {
        let gregorian = Calendar(identifier: .gregorian)
        let parts = month.split(separator: "-")
        guard parts.count == 2, let year = Int(parts[0]), let number = Int(parts[1]),
            let date = gregorian.date(from: DateComponents(year: year, month: number))
        else { return month }
        return date.formatted(
            Date.FormatStyle(locale: locale, calendar: gregorian).month(.wide).year())
    }

    /// Fetches this month's summary, and the month shown if it is another.
    public func load() async {
        await fetch(month: "")
        if let month = selectedMonth, month != currentMonth { await fetch(month: month) }
    }

    /// Shows `month`, fetching it when it is not the one in hand.
    public func select(month: String) async {
        selectedMonth = month
        if summaries[month] == nil { await fetch(month: month) }
    }

    /// Limits `botID` to `cents` a month; 0 removes the limit.
    public func setBudget(botID: String, cents: Int64) async {
        await setBudgets(bots: [
            .with {
                $0.botID = botID; $0.cents = cents
            }
        ])
    }

    /// Limits the whole team to `cents` a month; 0 removes the limit.
    public func setOrgBudget(cents: Int64) async {
        await setBudgets(orgCents: cents, bots: [])
    }

    /// Lets the paused `botID`, or the whole team for an empty id, work again.
    public func resume(botID: String) async {
        do {
            apply(try await client.resume(botID: botID))
            error = nil
        } catch {
            self.error = error.localizedDescription
        }
    }

    /// Applies a summary the core answered with elsewhere, such as the
    /// Inbox's Resume.
    func apply(_ summary: Hearth_V1_SummaryResponse) {
        // Only the current month is ever answered without being asked for.
        currentMonth = summary.month
        summaries[summary.month] = summary
        if !summary.months.isEmpty { months = summary.months }
    }

    func handle(_ event: Hearth_V1_SubscribeResponse) async {
        switch event.event {
        case .usageTick(let tick):
            apply(tick)
        case .resync:
            summaries = [:]
            await load()
        default:
            break
        }
    }

    private func fetch(month: String) async {
        do {
            let response = try await client.usageSummary(month: month)
            if month.isEmpty {
                apply(response)
            } else {
                summaries[response.month] = response
            }
            error = nil
        } catch {
            self.error = error.localizedDescription
        }
    }

    private func setBudgets(orgCents: Int64? = nil, bots: [Hearth_V1_BotBudget]) async {
        do {
            let cents = orgCents ?? current?.orgBudgetCents ?? 0
            apply(try await client.setBudgets(orgCents: cents, bots: bots))
            error = nil
        } catch {
            self.error = error.localizedDescription
        }
    }

    /// A tick replaces the bot's figures and the team's in the month it names;
    /// a month not fetched yet is left for its fetch.
    private func apply(_ tick: Hearth_V1_UsageTick) {
        guard var summary = summaries[tick.month] else { return }
        if let index = summary.bots.firstIndex(where: { $0.botID == tick.bot.botID }) {
            summary.bots[index] = tick.bot
        } else {
            summary.bots.append(tick.bot)
        }
        summary.org = tick.org
        summaries[tick.month] = summary
    }
}
