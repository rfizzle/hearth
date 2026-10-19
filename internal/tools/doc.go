// Package tools is where a bot's tool calls are offered and checked: the
// definitions a model may call, a registry of them, and the dispatcher that
// stands between a call and its definition.
//
// Responsibility: hold each definition's schema, description, capability,
// tier, executor and describer; list for one bot only the definitions its
// grants allow; and, before running a call, check its capability against the
// grants and, for a files capability, resolve the requested path against the
// folders that capability is granted in. The tools themselves live in the
// files and web packages below this one.
//
// Invariants the types do not hold:
//   - Every call goes through Dispatcher.Execute. A definition's executor
//     trusts that the check ran and that a files call's path is already
//     resolved inside a granted folder.
//   - A refusal is an error wrapping ErrRefused whose text is one line the
//     model can act on; it never runs the executor.
//   - Only a grant in allowed mode lets a call run. A grant in ask mode, or
//     an act-tier definition, is refused, because no approval gate is
//     installed yet.
//
// Seams it is handed: the definitions, the bot's grants and the folder its
// relative paths start from, all plain values. It imports org and
// workspace, and no model provider.
package tools
