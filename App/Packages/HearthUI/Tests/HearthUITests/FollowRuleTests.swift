// When the transcript follows new rows and when it leaves the reader alone.

import Testing

@testable import HearthUI

@Test func atTheBottomTheTranscriptFollows() {
    #expect(FollowRule.follows(wasFollowing: false, distanceFromBottom: 0, contentChanged: false))
    #expect(
        FollowRule.follows(
            wasFollowing: false, distanceFromBottom: FollowRule.slack, contentChanged: false))
}

@Test func scrollingUpStopsTheFollowing() {
    #expect(!FollowRule.follows(wasFollowing: true, distanceFromBottom: 300, contentChanged: false))
}

@Test func aNewRowKeepsWhateverTheReaderChose() {
    // The row grows the content, so the distance grows with no scrolling.
    #expect(FollowRule.follows(wasFollowing: true, distanceFromBottom: 120, contentChanged: true))
    #expect(!FollowRule.follows(wasFollowing: false, distanceFromBottom: 120, contentChanged: true))
}

@Test func aRunFoldingIsNotTheReaderScrolling() {
    // Folding shrinks the content under a reader who is following.
    #expect(FollowRule.follows(wasFollowing: true, distanceFromBottom: 0, contentChanged: true))
}

@Test func scrollingBackDownFollowsAgain() {
    #expect(FollowRule.follows(wasFollowing: false, distanceFromBottom: 4, contentChanged: false))
}
