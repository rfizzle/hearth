// HearthNative holds the native adapters (Keychain, Shortcuts, the global
// shortcut, the on-device model) and the server the core calls them through,
// NativeServer, which today reads the text of picture pages and images.

/// The package's version of the native surface.
public enum HearthNative {
    /// Bumped when the native tool surface changes.
    public static let version = 1
}
