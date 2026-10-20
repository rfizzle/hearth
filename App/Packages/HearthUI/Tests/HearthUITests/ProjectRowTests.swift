// A project's sidebar row: the light's tone follows the core's light, an
// active project with no tasks and a done one show none, and the row says
// its progress and its light in words.

import HearthDesign
import HearthKit
import Testing

@testable import HearthUI

@Test func aProjectRowsLightFollowsTheCore() {
    let states = ProjectRow.PreviewState.allCases
    #expect(
        states.map { ProjectRow.tone(of: $0.project) } == [
            .positive, .attention, .negative, nil, nil,
        ])
    #expect(
        ProjectRow.spoken(ProjectRow.PreviewState.yellow.project) == "3 of 7 done, Waiting on you")
    #expect(ProjectRow.spoken(ProjectRow.PreviewState.noTasks.project) == "No tasks yet")
    #expect(ProjectRow.spoken(ProjectRow.PreviewState.done.project) == "Done")
}
