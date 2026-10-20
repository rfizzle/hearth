# hearth · documents

Five files that are meant to stay true, one folder that is meant to go stale.

| File | Question it answers | When it changes |
|---|---|---|
| `product.md` | What is hearth, for whom, and what will it never do? | When the product changes |
| `model.md` | What are the things in the system and how does work flow between them? | When behaviour changes; code cites its headings |
| `architecture.md` | What are the big shapes, and why those? | When a shape changes |
| `design.md` | What does hearth look like and why? | When the look changes; code cites its headings |
| `decisions.md` | What was decided, when, and why; what is still open? | When a decision is made |
| `research/` | Dated snapshots: prior art, platform survey, the first UX sketch, what was lifted from shhh, the helper and on-device spike results, the ChatGPT sign-in spike, the projects-pattern survey, the Shortcuts spike, the one-employee slice run on a Mac and the person's live checklist | Never; a new snapshot gets a new dated file |

Rules that keep the surface small:

- A fact lives in one place. Working guidance (how to add a view, how to change the proto, how to release) is a skill in `.agents/skills`, not a document here. Where the code is and what the traps are is `AGENTS.md`. Planning is not in this repository.
- Documents name no code symbols; code cites documents by path and heading (`// See docs/model.md#3-assignment-delegation`). `make docs-check` fails on a citation that does not resolve and lists headings nothing cites, which is how dead sections get found.
- Anything time-bound (an API table, a competitor's feature list, a sketch of a screen) goes in `research/` with the date in the filename and is not edited afterwards.
