// Tests for the usage pieces: the headline in dollars or tokens, the budget
// tone rule, the cents a typed amount means, and the dollar words.

import Foundation
import HearthDesign
import HearthKit
import Testing

@testable import HearthUI

@MainActor
@Test func aPricedMonthHeadlinesDollarsAndAnUnpricedOneTokens() {
    let priced = UsageSummary(previewState: .priced)
    #expect(priced.headline == UsageSummary.dollars(micros: 4_200_000))
    #expect(priced.headline.contains("4.20") || priced.headline.contains("4,20"))
    let unpriced = UsageSummary(previewState: .unpriced)
    #expect(unpriced.headline.hasSuffix("tokens"))
    #expect(!unpriced.headline.contains("$"))
    #expect(unpriced.caption.contains("no price"))
}

@MainActor
@Test func theBudgetBarIsQuietUntilNineTenthsThenAttentionThenNegative() {
    #expect(UsageSummary(previewState: .priced).tone == .neutral)
    #expect(UsageSummary(previewState: .nearLimit).tone == .attention)
    #expect(UsageSummary(previewState: .paused).tone == .negative)
    #expect(UsageSummary(previewState: .unpriced).tone == .neutral)
}

private let americanEnglish = Locale(identifier: "en_US")

@MainActor
@Test func dollarsAreWrittenWholeWhenWholeAndWithCentsOtherwise() {
    #expect(UsageSummary.dollars(cents: 2000, locale: americanEnglish) == "$20")
    #expect(UsageSummary.dollars(cents: 1250, locale: americanEnglish) == "$12.50")
    #expect(UsageSummary.dollars(micros: 18_400_000, locale: americanEnglish) == "$18.40")
}

@MainActor
@Test func aTypedAmountBecomesCentsAndBlankMeansNoLimit() {
    #expect(BudgetField.cents(from: "20") == 2000)
    #expect(BudgetField.cents(from: " $12.50 ") == 1250)
    #expect(BudgetField.cents(from: "") == 0)
    #expect(BudgetField.cents(from: "twenty") == nil)
    #expect(BudgetField.cents(from: "-5") == nil)
    #expect(BudgetField.text(cents: 2000) == "20")
    #expect(BudgetField.text(cents: 1250) == "12.50")
    #expect(BudgetField.text(cents: 0) == "")
}
