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
