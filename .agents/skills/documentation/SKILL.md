---
name: documentation
description: hearth's delta on project-docs — docs/model.md as the document code cites, the citation example, and where the model reads a rule (a definition's schema or description, a toolbox line, a framing paragraph). Use alongside project-docs when writing a comment that cites docs/, or when a change moves what the model is told. The general rules (where a fact belongs, the citation convention, what a comment has to earn, work-item ids) are project-docs, not this skill.
---

# Documentation: hearth's delta

`project-docs` is the standard for the document set and for comments; read it
first. This skill holds only what hearth adds to it.

## What code cites

`docs/model.md` answers "what are the things, and how does work flow between
them?", and it is the document hearth's code cites most:

```go
// Act-tier tools always go through the approval gate: hearth cannot know
// whether a tool changed something outside the workspace, so it assumes it did.
// See docs/model.md#3-assignment-delegation.
```

## Where the model reads it

A tool, an argument, a refusal or a grant is something the model has to be
told, and it is told through the definition's schema or description, a toolbox
line, or a prompt paragraph (the fixed framing in `internal/framing`). The base
prompt names no tool: the toolset depends on the bot's grants, and a model
promised a tool it lacks tries to use it.

The line the model reads says what to do. The reason it says so goes in the
doc the line cites, like any other decision. The commonest failure is the
first channel written as a label — a property the model can see and cannot
judge when to pass:

```go
// Before: says what it is. The model passes it always or never.
"steps": {"type": "integer", "description": "Optional number of steps"},

// After: says when to pass it, and when not to.
"steps": {"type": "integer", "description": "Optional number of steps this task breaks into (max 20). Pass it when you can name the steps up front: the agent's lane then shows progress against it instead of a spinner. Leave it out rather than guessing — an invented denominator is worse than none."},
```

## Before you finish

- Does the change move what the model reads — a definition's schema or
  description, a toolbox line, a framing paragraph? Then say which, and cite
  the section that holds the reason; if it moves none, say that.
