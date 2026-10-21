// The org's projects: the API tier launch in flight with one task waiting on the owner,
// a move with no tasks yet, a finished tax pack and a proposal from the Chief
// of Staff that waits for the owner's click.

extension Fixtures {
    /// A project with no tasks: the light is none.
    static let noTasks = Hearth_V1_ProjectStatus.with {
        $0.light = .none
        $0.lightLabel = "No tasks yet"
    }

    /// Four projects: two active, one done, one proposed.
    public static let projects: [Hearth_V1_Project] = [
        .with {
            $0.id = "project-launch"
            $0.name = "Launch the API tier"
            $0.brief = .with {
                $0.goal = "Launch the new API tier to our customers."
                $0.context =
                    "400 customers are on the starter plan. No pricing page or launch email yet."
                $0.doneLooksLike = "A pricing page, a launch email and a help article."
                $0.doneWhen = "The launch email is in 400 inboxes."
            }
            $0.state = .active
            $0.targetDate = "2026-11-20"
            $0.folderPath = workspacePath + "/Projects/Launch the API tier"
            $0.conversationID = "conversation-project-launch"
            $0.status = .with {
                $0.tasksTotal = 7
                $0.tasksDone = 3
                $0.tasksWorking = 2
                $0.tasksWaitingOwner = 1
                $0.tasksCancelled = 1
                $0.light = .yellow
                $0.lightLabel = "Waiting on you"
            }
            $0.createdAt = time(1)
            $0.activatedAt = time(1)
            $0.updatedAt = time(12)
        },
        .with {
            $0.id = "project-office"
            $0.name = "Move the office"
            $0.brief = .with {
                $0.goal = "Move the team to the new office on Canal Road."
                $0.context = "The lease here ends in January."
                $0.doneLooksLike = "Desks, network and post all working at the new address."
                $0.doneWhen = "Everyone works from Canal Road."
            }
            $0.state = .active
            $0.folderPath = workspacePath + "/Projects/Move the office"
            $0.conversationID = "conversation-project-office"
            $0.status = noTasks
            $0.createdAt = time(2)
            $0.activatedAt = time(2)
            $0.updatedAt = time(2)
        },
        .with {
            $0.id = "project-tax"
            $0.name = "Spring tax pack"
            $0.brief = .with {
                $0.goal = "Gather what the accountant needs for the spring return."
                $0.context = "Receipts are in the shared folder, invoices in the books."
                $0.doneLooksLike = "One folder with every document, listed."
                $0.doneWhen = "The accountant confirms she has everything."
            }
            $0.state = .done
            $0.folderPath = workspacePath + "/Projects/Spring tax pack"
            $0.conversationID = "conversation-project-tax"
            $0.status = .with {
                $0.tasksTotal = 5
                $0.tasksDone = 5
                $0.light = .green
                $0.lightLabel = "All tasks done"
                $0.allTasksDone = true
            }
            $0.createdAt = time(3)
            $0.activatedAt = time(3)
            $0.doneAt = time(10)
            $0.updatedAt = time(10)
        },
        .with {
            $0.id = "project-website"
            $0.name = "Refresh the website"
            $0.brief = .with {
                $0.goal = "Bring the company website up to date."
                $0.context = "The customer stories stop in 2024 and the contact form is broken."
                $0.doneLooksLike = "New customer stories, working form, same look."
                $0.doneWhen = "The new site is live."
            }
            $0.state = .proposed
            $0.status = noTasks
            $0.proposedByBotID = chiefOfStaff.id
            $0.proposedInConversationID = chiefOfStaff.dmConversationID
            $0.createdAt = time(13)
            $0.updatedAt = time(13)
        },
    ]
}

extension Fixtures {
    /// What the model proposes for "We are launching our new API tier next
    /// month": two concerns, one with a suggestion and one without.
    public static let projectDraft = Hearth_V1_DraftProjectResponse.with {
        $0.name = "Launch the API tier"
        $0.brief = .with {
            $0.goal = "Launch the new API tier to our customers."
            $0.context =
                "400 customers are on the starter plan. No pricing page or launch email yet."
            $0.doneLooksLike = "A pricing page, a launch email and a help article."
            $0.doneWhen = "The launch email has gone out."
        }
        $0.targetDate = "2026-11-20"
        $0.concerns = [
            .with {
                $0.field = "done_when"
                $0.concern = "\u{201C}Gone out\u{201D} is hard to check. Try a number or a date."
                $0.suggestion = "The launch email has reached every customer on the starter plan"
            },
            .with {
                $0.field = "name"
                $0.concern =
                    "The name reads like the goal. A short name is easier to find in the sidebar."
            },
        ]
    }

    /// The rule for a date before today, in the core's words. The preview
    /// client computes only this one; the core's other rules are its own.
    static let pastDate = Hearth_V1_Concern.with {
        $0.field = "target_date"
        $0.concern = "That date has already passed."
    }
}
