# TASK-006 — Help Command Search/Filter Functionality

## Assignment
- **Assignee:** Dev-C
- **Priority:** MEDIUM (P3 from backlog)
- **Estimated:** M (2-3 days)
- **Source:** DIRECTION.md Backlog P3

---

## Objective
Add search and filter functionality to the bot's help command to improve user experience when discovering available commands.

---

## Requirements

### Functional Requirements
1. **Search by keyword:** Users can type `/help search <keyword>` to find commands matching the keyword
2. **Filter by category:** Users can type `/help filter <category>` to show only commands in a category
3. **Case-insensitive matching:** Search should be case-insensitive
4. **Fuzzy matching (optional):** Consider fuzzy matching for typo tolerance
5. **Results formatting:** Clear, numbered list with command name, description, and usage

### Non-Functional Requirements
- Response time < 100ms for search/filter operations
- Maintain backward compatibility with existing `/help` command
- Clear "no results" message when search returns empty

---

## Technical Context

### Current Help Command
- Location: `internal/bot/handler/` or similar (verify during implementation)
- Currently shows static list of all commands
- Categories likely include: market, analysis, alerts, settings, etc.

### Implementation Notes
1. Parse subcommand from message text
2. Filter command registry based on search/filter criteria
3. Return formatted results to user

---

## Acceptance Criteria
- [ ] `/help search <keyword>` returns matching commands
- [ ] `/help filter <category>` returns commands in that category
- [ ] `/help` (no args) still works as before (backward compatibility)
- [ ] Empty results show helpful "no commands found" message
- [ ] Unit tests for search/filter logic
- [ ] Manual testing confirms expected behavior

---

## Related
- Similar to: TASK-011 (multi-language support) — could share command metadata structure
- Part of: UX Improvement initiatives

---

*Assigned by: TechLead-Intel (Loop #64)*
*Task from: DIRECTION.md Backlog P3*
