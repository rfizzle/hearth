// ContentView shows the supervisor's status until the helper is ready, then
// the hello screen. It streams again each time a new helper comes up.

import HearthFeatures
import SwiftUI
import os

struct ContentView: View {
    @Environment(HelperSupervisor.self) private var supervisor
    @Environment(HelloStore.self) private var store
    private let log = Logger(subsystem: "com.rfizzle.hearth", category: "stream")

    var body: some View {
        Group {
            if supervisor.client == nil {
                Text(supervisor.status)
            } else {
                HelloScreen(store: store)
            }
        }
        .frame(minWidth: 480, minHeight: 240)
        .task(id: supervisor.generation) {
            guard let client = supervisor.client else { return }
            log.notice("stream started")
            await store.stream(from: client, count: 40)
            log.notice("stream finished")
        }
        .onChange(of: store.transcript.isEmpty) { _, isEmpty in
            if !isEmpty { log.notice("first word shown") }
        }
    }
}
