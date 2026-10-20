// Package tools is where a bot's tool calls are offered and checked: the
// definitions a model may call, a registry of them, and the dispatcher that
// stands between a call and its definition.
//
// Responsibility: hold each definition's schema, description, capability,
// tier, executor and describer; list for one bot only the definitions its
// grants give, asking or not; and, before running a call, check its
// capability against the grants and, for a files capability, resolve the
// requested path against the folders that capability is granted in, then
// say whether it runs now or waits for the owner. The tools themselves live
// in the files and web packages below this one.
//
// Invariants the types do not hold:
//   - Every call goes through Dispatcher.Execute or ExecuteApproved. A
//     definition's executor trusts that the check ran and that a files
//     call's path is already resolved inside a granted folder.
//   - A refusal is an error wrapping ErrRefused whose text is one line the
//     model can act on; it never runs the executor.
//   - Execute runs only a call whose verdict is Auto. A call under an
//     ask-mode grant, or to a tool with no standing, is Gated unless an
//     allowance for the conversation covers it, and runs only through
//     ExecuteApproved after the owner decides; that skips the verdict and
//     keeps the grant and path checks, so an approval never reaches past the
//     folders the bot is granted at the moment it runs.
//
// Seams it is handed: the definitions, the bot's grants, the conversation's
// allowances and the folder its relative paths start from, all plain values.
// It imports org, org/inbox and workspace, and no model provider.
package tools
