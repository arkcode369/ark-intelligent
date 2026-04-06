# Task Distribution Guidelines

> Task assignment rules and guidelines for the agent workflow system.

---

## Priority Levels

| Priority | Description | Response Time |
|----------|-------------|---------------|
| **Critical** | Blocks release, fix immediately (any agent) | Immediate |
| **High** | Core functionality, should complete within 24h | Within 24h |
| **Medium** | Important but not blocking, complete within 3 days | Within 3 days |
| **Low** | Nice to have, complete within 1 week | Within 1 week |

---

## Agent Assignment Rules

### Dev-A (Backend/Core)
- Database/storage changes
- Core service logic
- API integrations
- Performance optimizations
- Context timeout fixes
- Type assertion corrections

### Dev-B (Features/Handlers)
- Telegram handlers
- New commands
- Feature implementation
- Integration work
- User-facing features

### Dev-C (Testing/Infrastructure)
- Test coverage improvements
- CI/CD improvements
- Refactoring
- Documentation
- Infrastructure setup

### QA (Quality Assurance)
- Code review
- Bug verification
- Test plan review
- Merge approval
- Validation gate

---

## Effort Estimation Guidelines

| Effort | Description | Typical Work |
|--------|-------------|--------------|
| **1h** | Quick fixes, documentation updates, single-file changes | ~50 lines |
| **2h** | Small features, simple refactors | ~100 lines |
| **4h** | Medium features, multiple file changes | ~200 lines |
| **1d** | Complex features, new modules | ~500 lines |
| **2d** | Substantial changes, architecture work | ~1000 lines |
| **1w** | Major features, migrations | Multiple modules |

---

## Task Lifecycle

```
┌──────────────┐    ┌──────────────┐    ┌──────────────┐
│   Research   │ -> │  Coordinator │ -> │    Agent     │
│ Create Spec  │    │   Assign     │    │   Claim      │
└──────────────┘    └──────────────┘    └──────────────┘
                                               │
                                               v
┌──────────────┐    ┌──────────────┐    ┌──────────────┐
│  Coordinator │ <- │     QA       │ <- │    Agent     │
│    Archive   │    │    Review    │    │  Implement   │
└──────────────┘    └──────────────┘    └──────────────┘
```

### Steps:

1. **Research** creates task spec in `.agents/tasks/pending/`
2. **Coordinator** assigns to agent via `STATUS.md`
3. **Agent** claims task and moves to `in-progress/`
4. **Agent** completes work and creates PR
5. **QA** reviews and approves
6. **Coordinator** merges and archives task to `completed/`

---

## Task Handoff Procedures

### Agent-to-Agent Handoff
- Update `STATUS.md` with new agent assignment
- Document progress in task file's `## Progress Log` section
- Move task file to new agent's pending queue if needed
- Notify receiving agent via status update

### Agent-to-QA Handoff
- Create PR with clear description linking to task
- Update task status to "In Review"
- Add PR link to task file
- QA becomes assignee in `STATUS.md`

---

## Escalation Rules

### Time-Based Escalation

| Duration | Action | Escalation Target |
|----------|--------|-------------------|
| Blocked > 4 hours | Document blocker in task, update STATUS.md | Coordinator |
| Blocked > 8 hours | Request reassignment or additional resources | TechLead |
| Overdue > 2x estimate | Re-evaluate scope or split task | Coordinator + Research |

### Priority-Based Escalation

- **Critical bug**: Any agent can be immediately reassigned
- **Security issue**: Bypass normal queue, immediate coordinator attention
- **Test failure blocking CI**: Escalate to Dev-C + Coordinator

### Escalation Format

When escalating, update the task file with:

```markdown
## Escalation Log
- 2026-04-06T10:00:00Z: Blocked by X, escalated to Coordinator
```

---

## Conflict Resolution

### Multiple Agents on Same Task
- First claim (STATUS.md update) wins
- Coordinator has override authority
- Disputes resolved by TechLead

### Priority Disputes
- Critical > High > Medium > Low
- Within same priority: FIFO by creation date
- Coordinator can reprioritize with justification

---

*Last updated: 2026-04-06*
