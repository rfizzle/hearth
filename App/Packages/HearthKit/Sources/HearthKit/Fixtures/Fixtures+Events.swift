// A scripted subscription that replays Dana's reply in the order the core
// sends one: the run working, the message opened empty and streaming, text in
// deltas, the message replaced whole and final, the run done. A heartbeat
// leads, as one would on a quiet stream. Sequences start at 1, as they do on
// every helper launch.

extension Fixtures {
    /// The reply's text, one delta at a time; the DM shows it after the first.
    public static let replyDeltas = [
        "I've written the brief and saved it in my folder. ",
        "It covers how a heat pump works, what a 1930s house needs first ",
        "(loft and wall insulation, bigger radiators), and the grants on offer.",
    ]

    /// The events PreviewClient's subscribe replays, ascending by sequence.
    public static let replyEvents: [Hearth_V1_SubscribeResponse] = {
        var opened = streamingReply
        opened.content = ""
        var finished = streamingReply
        finished.content = replyDeltas.joined()
        finished.streaming = false
        var done = workingRun
        done.state = .done
        done.endedAt = time(15)

        var events: [Hearth_V1_SubscribeResponse.OneOf_Event] = [
            .heartbeat(Hearth_V1_Heartbeat()),
            .runChanged(.with { $0.run = workingRun }),
            .messageUpserted(.with { $0.message = opened }),
        ]
        events += replyDeltas.map { text in
            .messageDelta(
                .with {
                    $0.conversationID = dmConversation.id
                    $0.messageID = streamingReply.id
                    $0.runID = workingRun.id
                    $0.text = text
                })
        }
        events += [
            .messageUpserted(.with { $0.message = finished }),
            .runChanged(.with { $0.run = done }),
        ]
        return events.enumerated().map { index, event in
            .with {
                $0.sequence = UInt64(index + 1)
                $0.event = event
            }
        }
    }()
}

extension Fixtures {
    /// A second script: Dana parks at the delete and resumes when the owner
    /// allows it. The run waits, the request lands in the thread and the Inbox;
    /// the item closes, the decision and the result land in the thread, and the
    /// run works on to the end. Sequences start at 1.
    public static let parkEvents: [Hearth_V1_SubscribeResponse] = {
        var waiting = workingRun
        waiting.state = .waitingOwner
        var closed = approval
        closed.state = .closed
        closed.decision = .allowOnce
        closed.outcome = "Allowed once"
        closed.closedAt = time(17)
        var done = workingRun
        done.state = .done
        done.endedAt = time(18)

        let events: [Hearth_V1_SubscribeResponse.OneOf_Event] = [
            .runChanged(.with { $0.run = waiting }),
            .messageUpserted(.with { $0.message = approvalRequest }),
            .inboxChanged(.with { $0.item = approval }),
            .inboxChanged(.with { $0.item = closed }),
            .messageUpserted(.with { $0.message = approvalDecision }),
            .messageUpserted(.with { $0.message = deleteResult }),
            .runChanged(.with { $0.run = workingRun }),
            .runChanged(.with { $0.run = done }),
        ]
        return events.enumerated().map { index, event in
            .with {
                $0.sequence = UInt64(index + 1)
                $0.event = event
            }
        }
    }()
}

extension Fixtures {
    /// A third script for slice B: a run finishes and the usage ticks, Dana
    /// remembers a note and the owner edits then deletes another, and a
    /// handbook page is saved. Sequences start at 1.
    public static let memoryEvents: [Hearth_V1_SubscribeResponse] = {
        var remembered = notes[0]
        remembered.id = "note-new"
        remembered.pinned = false
        remembered.text = "Prefers a one-line answer before the detail."
        remembered.createdAt = time(20)
        var edited = notes[1]
        edited.text = "The owner's house is a 1930s semi in Leeds."

        let events: [Hearth_V1_SubscribeResponse.OneOf_Event] = [
            .usageTick(
                .with {
                    $0.bot = usageSummary(month: "").bots[0]
                    $0.org = usageSummary(month: "").org
                    $0.month = currentMonth
                }),
            .noteChanged(
                .with {
                    $0.note = remembered
                    $0.botID = bot.id
                }),
            .noteChanged(
                .with {
                    $0.note = edited
                    $0.botID = bot.id
                }),
            .noteChanged(
                .with {
                    $0.note = .with { $0.id = notes[4].id }
                    $0.deleted = true
                    $0.botID = bot.id
                }),
            .handbookChanged(.with { $0.page = handbookPages[1] }),
            .handbookChanged(
                .with {
                    $0.page = .with { $0.id = handbookPages[1].id }
                    $0.deleted = true
                }),
        ]
        return events.enumerated().map { index, event in
            .with {
                $0.sequence = UInt64(index + 1)
                $0.event = event
            }
        }
    }()
}

extension Fixtures {
    /// A fourth script: the owner marks a project done, so the project
    /// arrives with its new state. Sequences start at 1.
    public static let projectEvents: [Hearth_V1_SubscribeResponse] = {
        var finished = projects[0]
        finished.state = .done
        finished.doneAt = time(30)
        finished.updatedAt = time(30)
        return [
            .with {
                $0.sequence = 1
                $0.event = .projectChanged(.with { $0.project = finished })
            }
        ]
    }()
}

extension Fixtures {
    /// Another script: the core rewrote the DM's summary after a run, so the
    /// conversation arrives with its new summary. Sequences start at 1.
    public static let summaryEvents: [Hearth_V1_SubscribeResponse] = {
        var changed = dmConversation
        changed.summary =
            summaryText
            + " The brief is written and saved in her folder; the grants are still open."
        return [
            .with {
                $0.sequence = 1
                $0.event = .conversationChanged(.with { $0.conversation = changed })
            }
        ]
    }()
}
