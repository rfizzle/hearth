// Catalog lists every page the gallery shows. Each area has its own file, so
// a story that restyles one area edits that file and nothing else.

@MainActor
enum Catalog {
    /// Built once: the screens hold stores that load in the background.
    static let pages: [GalleryPage] =
        design + onboarding + hire + conversation + inbox + widgets + access + inspector + settings
        + screens + projects
        + [motion]
}
