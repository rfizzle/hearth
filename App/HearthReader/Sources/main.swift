// HearthReader is the reading service: an XPC service inside Hearth.app,
// sandboxed with nothing granted, that reads the text of the files the app
// hands it open. Only the app can reach it. A call that runs past its time
// ends this process, which the app also kills from its side; the next call
// starts a fresh one.
// See docs/decisions.md#hearth--decisions

import Foundation
import HearthReading

final class ServiceDelegate: NSObject, NSXPCListenerDelegate {
    func listener(
        _ listener: NSXPCListener, shouldAcceptNewConnection connection: NSXPCConnection
    ) -> Bool {
        connection.exportedInterface = Reading.interface()
        connection.exportedObject = Reader(overrun: { _exit(EXIT_FAILURE) })
        connection.resume()
        return true
    }
}

let delegate = ServiceDelegate()
let listener = NSXPCListener.service()
listener.delegate = delegate
listener.resume()
