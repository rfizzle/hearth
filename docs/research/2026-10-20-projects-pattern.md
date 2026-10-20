# The unit above a task

> Snapshot from 2026-10-20: a survey of how work tools and agent products model the layer above a single task, made when the owner asked for a pattern ("Project, Goal, OKRs"). Not maintained. Cells marked (k) are from general knowledge and were not verified against a page in this run; Linear, Basecamp, Asana, Atlassian, Devin, OpenAI and Anthropic pages were reachable only through search summaries.

## What hearth has

An org with one owner; bots in teams with one lead each, and org-level bots such as the Chief of Staff; DMs, team channels and task threads; a Task as the unit of delegation with a full brief, states, budgets and deliverables in the Workspace with one folder per team; a Schedule to repeat a task; a strictly hierarchical assignment tree. Nothing groups related work across teams over time.

## Survey

| Product | Units above a task | Key fields | Progress | How it ends |
|---|---|---|---|---|
| Things | Area (ongoing) > Project > Heading | Project: title, notes, deadline, tags (k) | Pie that fills as to-dos complete (k) | Completed and logged |
| OmniFocus | Folder > Project (sequential, parallel, single-action); Perspectives | Status: active, on hold, completed, dropped; review interval (k) | Counts of available actions; the review habit | Completed or dropped |
| Basecamp | Project (to-do lists, message board, docs, chat); Hill Chart per list | Assignee and due date per to-do | Hill Chart placed by hand | Archived (k) |
| Linear | Initiative > Project > Milestone; Cycle as a time box | Project: one lead, members, dates, status, spans teams; Initiative: curated projects plus a document | Progress 0 to 1; lead posts health (on track, at risk, off track) | Completed or canceled (k); initiative archived |
| Asana | Goal (nests); Portfolio; Project > Section | Goal: owner, period, status, progress source, target | Green, yellow, red; percent of target | Achieved, missed, partial, dropped |
| Notion templates | Goals > Projects > Tasks as linked databases | Whatever the owner adds | Rollup of tasks checked | A status property |
| Todoist | Project > Section | Name, colour, sections | Completed counts | Archive only |
| Jira | Epic > Initiative or Theme | Epic: summary, dates, children | Donut of resolved children | Done when children resolve |
| GTD horizons | Actions; Projects (outcomes needing more than one action); Areas of responsibility; goals; vision; purpose | A project is an outcome plus a next action | The weekly review | A project ends when its outcome is true; areas never end |
| OKRs (Doerr) | Objective with three to five Key Results, quarterly (k) | Owner, quarter, score 0 to 1 (k) | Score per key result; 0.7 is success for stretch goals | Graded and re-set. Critics: overhead for tiny teams, stale within a quarter, built for large-org alignment |
| Devin | Session (one task); Playbook (reusable procedure); Knowledge | Playbook: steps, specifics, postconditions | Session status | The playbook outlives the session |
| Claude and ChatGPT Projects | Project = instructions, files, scoped chats | Instructions, files, chats | None; a container | Never ends; deleted by hand |
| Manus, Claude Code plans | A checklist file rewritten as the agent works; a plan approved before work | A checklist | The checklist itself | Done when the list is done |

Understood without training: Project, Folder and Area, a checklist, a shared folder of files, traffic lights, a due date. Needs explanation: Key Results and scoring, Hill Charts, Initiatives, Portfolios, Epics, Milestones, Cycles, sequential versus parallel, Playbooks.

Every product with a real project has the same core: a name, an outcome, a list of tasks, files, an owner. Progress is almost always a rollup of task states plus an optional hand-written health colour. Ongoing responsibilities are kept apart from finishable outcomes, and hearth's Teams already play the ongoing role. Projects end by completion or archive, with "done" and "dropped" distinguished.

## Three candidates

**A. Project.** A standing brief (goal, context, what done looks like), its tasks, a folder of deliverables, a conversation; status derived from tasks plus an optional date. Created by the owner, or proposed by the Chief of Staff and confirmed. Spans teams. Ends done or archived. A "Projects" sidebar section with a progress pie per row. Costs one entity and a project reference on Task. Borrows Things, Linear, Claude Projects, Basecamp.

**B. Goal with a measure.** An objective, one to three plain-language checks, tasks, a folder, a date or cadence. Owner-created. Closes as achieved, partly, missed or dropped. Costs a project plus check definitions and a rule for judging them, and invites the OKR overhead criticism for a one-person org. Borrows Asana and Doerr.

**C. Project inside an Area.** Two levels, as Things and GTD: an Area that never ends with a standing brief, budget and folder; Projects inside it. Costs the most and risks a second tree beside Teams, which already act as areas.

## Recommendation

Pattern A, "Project" as the owner-facing word, with one line borrowed from B: a "done when" sentence in the brief. No Key Results. Reasons: the word needs no training and ends by done or archived; it fills the named gap exactly; Teams already serve as areas; a one-person org needs no scoring, quarters or stretch targets, and a "done when" line gives the Chief of Staff and the leads a stopping condition. Progress is derived from task states, shown as a pie with "3 of 7 done" and one traffic light the system sets (waiting on the owner is yellow, a budget pause is red).

Questions left for the owner: who creates a project (owner only, or the Chief of Staff proposes and the owner confirms); where a project's output goes (a Projects folder in the Workspace, or inside team folders with a tag); whether a project has its own conversation or lives under the Chief of Staff DM with the brief pinned, and whether recurring schedules belong to projects or stay on teams.
