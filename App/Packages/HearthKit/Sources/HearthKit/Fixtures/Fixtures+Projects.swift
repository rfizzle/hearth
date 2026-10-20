// The org's projects: a launch in flight with one task waiting on the owner,
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
            $0.id = "project-newsletter"
            $0.name = "Launch the newsletter"
            $0.brief = .with {
                $0.goal = "Send the first issue of the studio newsletter."
                $0.context = "We have 400 subscribers from the old mailing list and no template."
                $0.doneLooksLike = "A template, a first issue and a sending schedule."
                $0.doneWhen = "The first issue is in 400 inboxes."
            }
            $0.state = .active
            $0.targetDate = "2026-11-20"
            $0.folderPath = workspacePath + "/Projects/Launch the newsletter"
            $0.conversationID = "conversation-project-newsletter"
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
                $0.goal = "Move the studio to the new unit on Canal Road."
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
                $0.goal = "Bring the studio website up to date."
                $0.context = "The portfolio stops in 2024 and the contact form is broken."
                $0.doneLooksLike = "New portfolio, working form, same look."
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
