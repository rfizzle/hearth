# The helper spike: Hearth.app streams words from a bundled hearthd

> Snapshot from 2026-10-19: what was built, measured and learned when the app first launched the Go helper and streamed from it. Not maintained.

## 1. Setup

- Host: macOS 27.0, Xcode 27.1, Swift 6.4, Go 1.27.1, buf 1.73.0, SwiftLint 0.65.1, Apple's `swift format` from the Xcode toolchain, `xcrun notarytool` 1.1.3, `vtool`, `stapler`. No code-signing identity is installed, and none is meant to be.
- GitHub's `macos-latest` was macOS 26.6 with Xcode 26.6, without buf or SwiftLint (the CI workflow installs both). The packages declare `swift-tools-version: 6.2` and macOS 26 so that toolchain can build them; no CI run had happened when this was written.
- The work ran in a Linux VM (arm64) over the Mac's checkout. Go builds, tests and `buf` ran natively in the VM; every Swift, codesign, notarytool, `open` and `osascript` command ran on the Mac through a shim in the same working directory. `buf generate` gave byte-identical output on both sides.

## 2. Transport: loopback TCP with a per-launch token, not a Unix socket

The plan was a Unix socket in a user-only directory with a peer-uid check. connect-swift 1.2.3 ships two HTTP clients, and neither can reach one:

- `URLSessionHTTPClient` (`open class`, `init(configuration: URLSessionConfiguration = .default)`) rides on URLSession, and Foundation has no Unix-socket API.
- `NIOHTTPClient` (`Libraries/ConnectNIO/Public/NIOHTTPClient.swift`) stores `private let host: String; private let port: Int`; its only initialiser is `init(host: String, port: Int? = nil, timeout: TimeInterval? = nil)` and it connects with `ClientBootstrap.connect(host:port:)`. There is no socket-path parameter, and using it would pull NIOCore, NIOHTTP1, NIOHTTP2, NIOPosix and NIOSSL into the app.

A socket would have meant a third `HTTPClientInterface` over Network.framework with hand-written HTTP/1.1 framing and Connect's envelope: more than the day the question allowed, and a transport hearth would own for good. D16 records the choice: the helper listens on `127.0.0.1:0`, prints the bound address as its first stdout line, and every request carries `Authorization: Bearer <token>`, a 32-byte random token minted per launch; middleware ahead of the router refuses anything else with Connect's `unauthenticated`, including unknown paths. The Swift side sets the header once in the URLSession configuration, so no interceptor was needed.

Consequences, as D16 says: the token replaces the peer-uid check; the URLSession client cannot stream both ways, so native tools will be served by a second server in the app; and the helper learns the app is gone from its stdin closing, because HTTP connections are per request. Killing the app with SIGKILL took the helper down within a second, which is the case the stdin lifeline exists for. `hearthd serve --socket <path>` still binds a Unix socket (mode 0600) and the contract test pings over it, so the socket path stays open if connect-swift gains a transport for it.

The token is passed in argv, which other local processes of the same user can read with `ps`. It could move to stdin, which is already a pipe from the app, when that matters.

## 3. Codegen

`buf.gen.yaml` pins four remote plugins, so `buf` is the only tool needed on Linux, the Mac and CI, and the Swift plugins need no Swift toolchain: `buf.build/protocolbuffers/go:v1.36.12`, `buf.build/connectrpc/go:v1.21.0`, `buf.build/apple/swift:v1.39.0`, `buf.build/connectrpc/swift:v1.2.3`. The generated Go and Swift are committed; `make proto-check` lints, checks breaking changes against master and fails on drift.

- The BSR had no `connectrpc/go` v1.21.1 plugin (its latest was v2.0.1); v1.21.0 output builds and runs against the v1.21.1 Go module.
- connect-go v2.0.0 was a week old. D17 stays on v1 and leaves the move to the migration tool, in its own change.
- `buf lint` STANDARD wants `PingRequest`/`PingResponse`-style names; the spike's `HelloRequest` and `HelloChunk` break two rules, ignored for that one file until the spike RPCs leave.
- The BSR rate-limits unauthenticated code generation, and the limit is tight. After a dozen generations in the first hour, `make proto-check` (one `buf generate`, four plugins) failed with `resource_exhausted: too many requests` on most runs: one passed at 22:43, the next three over six minutes were refused, the one at 22:49 passed. Every `make ci` runs it, so two gates within a few minutes of each other, from the VM and the Mac behind one address, can fail on it. `buf registry login` raises the limit; CI would need a BSR token as a secret, or local plugins.
- Generated Go is not goimports-clean, so it is left out of `make fmt` and `make fmt-check`; proto-check holds it to the generator instead.
- The generated Go is not excluded from astimate: the `exclude` key the configuration's comment describes is newer than the installed astimate (v0.7.2), which rejects the file when it is present. The gate passed with the two generated packages among the five it judged; its one heads-up was the two handler methods, tested through HTTP, which it counts as untested exports.

## 4. Timings

| What | Time |
|---|---|
| `make proto-check` (network, not rate-limited) | 0.8 s |
| `make swift-test`, cold (no `.build` anywhere) | 92 s |
| `make swift-test`, warm | 10 s |
| `make ci-swift`, warm | 14 s |
| `make app-build`, Swift warm | 17 s |
| Helper launched to first word on screen | 75 ms (launch 08.810, address 08.849, first word 08.885) |
| `open` to first word | under 2 s (the `open` timestamp had one-second resolution) |
| 40 words at one per 50 ms | 2.0 s, arriving one at a time, not in bursts |
| Helper killed to first word from its replacement | 1.10 s (one second of that is the deliberate restart delay) |
| App quit to helper gone | under 2 s |
| App killed with SIGKILL to helper gone | under 1 s |

No CI run happened before this was written.

## 5. Gotchas

- A SwiftPM resource is a `.bundle` beside the executable. In an app, `Bundle.module` looks in `Contents/Resources`, and with no Xcode project nothing puts it there: the bundle script copies every `*.bundle` from the build directory, or `HearthDesign`'s tokens are missing and the app aborts at launch.
- Ad-hoc `codesign --timestamp` succeeds silently; there is no timestamp to check until a Developer ID signature.
- The Go helper's `LC_BUILD_VERSION` says minos 13.0 while the app says 26.0. Notarisation accepts that.
- `swift test` builds each package in its own `.build`, so a cold run compiles swift-protobuf more than once; CI caches the `.build` directories.
- The root package has only app targets, and `swift test` fails on a package with no tests, so `make swift-test` builds the root and tests the five packages.
- Apple's `swift format` adds trailing commas to multi-line collections and SwiftLint's `trailing_comma` rule forbids them; the SwiftLint rule is off.
- `buf breaking --against '.git#branch=master'` needs master in the clone (`fetch-depth: 0` in CI), and buf drops an empty `core.fsmonitor` override when it runs git, so the Makefile passes `false` for that one call.
- The helper exits when its stdin closes. Run by hand from a terminal it stays up; started in the background of a script, its stdin is often `/dev/null` and it exits at once.
- Screen capture from the shim is refused, so "the window shows the words" was checked through the app's unified log (`log show --predicate 'subsystem == "com.rfizzle.hearth"'`), which records the first word on screen.

## 6. Signing and the second Mac

There is no Developer ID identity on this Mac and none is to be put there (D18). The story asked for a team id and a keychain profile; they were read as the five Actions secrets shhh's release already uses: `MACOS_SIGN_P12` and `MACOS_SIGN_PASSWORD` for the certificate, `MACOS_NOTARY_ISSUER_ID`, `MACOS_NOTARY_KEY_ID` and `MACOS_NOTARY_KEY` for an App Store Connect API key (the key, not a notarytool keychain profile, so CI needs no keychain state).

Done here: the bundle was built, ad-hoc signed helper first and then the app with the hardened runtime and a timestamp; `codesign -dv` showed `flags=0x10002(adhoc,runtime)` on both, `codesign --verify --deep --strict` passed, `vtool -show` found `LC_BUILD_VERSION` on the helper, and the signed app launched and streamed. `make notarize` without the three variables exited 1 naming them.

Not done: notarisation, stapling and a launch on a second Mac, because no certificate, no notary key and no second Mac were available. To finish: add the five secrets to the repository, run the Release workflow (a `v*` tag or a manual dispatch), download the `Hearth` artifact on a second Mac, unzip and open it, confirm Gatekeeper accepts it without a prompt and the words stream, and write the result in a new dated note.
