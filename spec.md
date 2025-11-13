# Specification: Issue #303 - Slash Commands Rendering Order

## Feature Title
Respect Workflow Template Command Ordering in Dashboard UI

## Problem Statement

Currently, slash commands displayed in the ACP dashboard are rendered in alphabetical/filesystem order, regardless of the intended sequence defined by workflow template authors. This breaks logical workflow sequences and prevents users from customizing command presentation.

**Impact**: Users cannot control how commands are organized in their workflows, limiting UX optimization and task flow clarity.

## Feature Description

Enable workflow authors to define custom command ordering in their workflow templates. The backend will parse this ordering specification and return commands in the specified sequence. The UI will display commands in the order provided by the backend, respecting the author's intent.

## User Stories

### US1: Workflow Author Defines Command Order
**As a** workflow author
**I want to** specify the order in which slash commands appear in the dashboard
**So that** I can present commands in a logical workflow sequence (e.g., Plan → Specify → Analyze)

**Acceptance Criteria:**
- I can create a `_order.yaml` file in `.claude/commands/` directory
- File format is simple YAML with a list of command IDs
- Backend automatically detects and applies this ordering
- My commands appear in the specified order in the UI

### US2: Backward Compatibility
**As a** platform user with existing workflows
**I want to** continue using workflows without modification
**So that** I don't experience breaking changes or forced migrations

**Acceptance Criteria:**
- Workflows without `_order.yaml` still work
- Commands display in alphabetical order (current behavior)
- No errors or warnings for existing workflows
- Migration is completely optional

### US3: Partial Command Ordering
**As a** workflow author
**I want to** specify order for only some commands
**So that** I don't need to maintain a complete list as my workflow evolves

**Acceptance Criteria:**
- I can list only priority commands in `_order.yaml`
- Unlisted commands are automatically appended in alphabetical order
- New commands automatically appear without requiring file updates
- Order is stable and predictable

### US4: Invalid Order Configuration Handling
**As a** platform operator
**I want to** invalid/malformed order specifications to be handled gracefully
**So that** they don't crash the system or hide commands

**Acceptance Criteria:**
- Malformed YAML is logged and ignored (fallback to alphabetical)
- Missing commands in order list don't cause errors
- Duplicate entries are deduplicated
- All commands are always included in final response

## Functional Requirements

### FR1: Order Specification Format
- **What**: Workflow templates support ordering via `.claude/commands/_order.yaml`
- **Format**:
  ```yaml
  commands:
    - project.plan
    - project.specify
    - project.analyze
    - project.clarify
    - project.tasks
  ```
- **Location**: Must be in `.claude/commands/` directory (same level as command `.md` files)
- **Scope**: Per-workflow configuration (each workflow can have different ordering)

### FR2: Backend Order Parsing
- **Where**: `components/backend/handlers/content.go::ContentWorkflowMetadata()`
- **Behavior**:
  1. Check if `_order.yaml` exists in workflow's `.claude/commands/` directory
  2. If exists, parse YAML and extract command ID list
  3. If not found, proceed with default ordering (alphabetical)
  4. Validate all listed commands exist (warn if missing)
  5. Return commands sorted according to specification
  6. Include any unlisted commands in alphabetical order after specified ones

### FR3: Order Application Logic
- **Input**: Map of commands by ID + order list from YAML
- **Process**:
  1. Create ordered list starting with specified commands (in order)
  2. Append remaining commands alphabetically
  3. Deduplicate if command appears in both list and files
  4. Verify no commands are lost
- **Output**: Commands slice sorted by specification

### FR4: Backward Compatibility
- **No order file exists**: Return commands in alphabetical order (current behavior)
- **No breaking changes**: API response format identical
- **Existing workflows**: Continue functioning without modification

### FR5: Error Handling
- **Missing order file**: Silently proceed with alphabetical order
- **Malformed YAML**: Log warning, use alphabetical fallback
- **Missing commands in order list**: Log info, include in alphabetical section
- **Duplicate entries**: Deduplicate (first occurrence wins)
- **Never**: Crash, return partial results, or hide commands

### FR6: Logging & Observability
- **Info level**: "Order file found for workflow X, applying custom ordering"
- **Info level**: "Workflow Y has N commands, M specified in order, L automatic"
- **Warn level**: "Order file for workflow X references missing command Z"
- **Debug level**: "Sorted commands: [id1, id2, id3, ...]"

## Non-Functional Requirements

### NFR1: Performance
- Order parsing overhead: **<10ms**
- File I/O: Single read of `_order.yaml`
- No impact on session creation time
- No caching issues (order loaded per metadata request)

### NFR2: Security
- Order file validation prevents directory traversal
- YAML parsing only (no code execution)
- Respects existing Kubernetes RBAC
- No sensitive data exposure in logs

### NFR3: Maintainability
- Clear, documented code with comments
- Type-safe Go implementation
- Proper error handling patterns
- Follows existing code style

### NFR4: Testability
- Unit tests for order parsing
- Unit tests for sort logic
- Integration tests with real workflows
- E2E tests in CI/CD pipeline

## Implementation Details

### Code Changes

**File: `components/backend/handlers/content.go`**

**New Function: `parseCommandOrder()`**
```go
func parseCommandOrder(workflowDir string) ([]string, error) {
    // Purpose: Read and parse _order.yaml file
    // Input: Workflow directory path
    // Output: Slice of command IDs in specified order, or error
    // Behavior:
    // - Check if _order.yaml exists
    // - Parse YAML structure: {commands: [id1, id2, ...]}
    // - Return command ID slice in order
    // - Log info/warn messages as appropriate
    // - Handle errors gracefully
}
```

**Modified Function: `ContentWorkflowMetadata()`**
```go
// Existing code: Read all command files from .claude/commands/
// Into: commandMap map[string]map[string]interface{}

// NEW: Parse ordering specification
orderedIds, err := parseCommandOrder(workflowDir)
if err != nil {
    log.Printf("Warning: failed to parse order file: %v", err)
}

// NEW: Apply ordering to commands slice
commands := sortCommandsByOrder(commandMap, orderedIds)

// Return commands in sorted order (existing return logic)
```

**New Function: `sortCommandsByOrder()`**
```go
func sortCommandsByOrder(
    commandMap map[string]map[string]interface{},
    orderedIds []string,
) []map[string]interface{} {
    // Purpose: Sort commands according to specification
    // Input: Map of all commands + ordered list of IDs
    // Output: Sorted slice of command objects
    // Behavior:
    // - Start with commands in orderedIds list (in order)
    // - Append remaining commands in alphabetical order
    // - Skip commands in order list that don't exist (warn)
    // - Deduplicate if command appears twice
    // - Return complete list with no commands missing
}
```

### Type Definitions

No new types required. Existing `WorkflowCommand` type remains unchanged:
```go
map[string]interface{}{
    "id":           string,
    "name":         string,
    "description":  string,
    "slashCommand": string,
    "icon":         string,
}
```

Commands are ordered by position in the returned slice, not by a dedicated field.

### API Response Format

No changes to API response format. Commands remain in the same JSON structure, but now in the correct order:

```json
{
  "commands": [
    {"id": "project.plan", "name": "Planning Workflow", ...},
    {"id": "project.specify", "name": "Specification", ...},
    {"id": "project.analyze", "name": "Analysis", ...}
  ],
  "agents": [...],
  "config": {...}
}
```

### Frontend Changes

**NO CHANGES REQUIRED** - Frontend already respects command order from backend.

The `WorkflowsAccordion` component renders commands via `.map()` without any sorting:
```typescript
workflowMetadata.commands.map((cmd) => {
  // Render in received order
})
```

## Acceptance Criteria

### Functional Acceptance

- ✅ `_order.yaml` file in `.claude/commands/` is parsed and applied
- ✅ Commands display in UI in order specified in `_order.yaml`
- ✅ Unlisted commands appear after specified commands in alphabetical order
- ✅ Workflows without `_order.yaml` display commands in alphabetical order (backward compatible)
- ✅ Invalid/missing order files handled gracefully (no crashes)
- ✅ All commands always included in response (nothing hidden)

### Testing Acceptance

- ✅ Unit tests pass for `parseCommandOrder()` function
- ✅ Unit tests pass for `sortCommandsByOrder()` function
- ✅ Integration test with real workflow file structure
- ✅ E2E test verifying UI displays correct order
- ✅ Backward compatibility test with existing workflows
- ✅ Edge case tests (empty order file, malformed YAML, missing commands)

### Code Quality Acceptance

- ✅ Code follows existing patterns in `handlers/content.go`
- ✅ Error handling is comprehensive and logged
- ✅ No tokens or sensitive data logged
- ✅ Type-safe implementation (no `panic()`)
- ✅ Comments explain logic clearly
- ✅ `gofmt`, `go vet`, `golangci-lint` all pass

### Documentation Acceptance

- ✅ Function comments with purpose, input, output
- ✅ Example `_order.yaml` format documented
- ✅ Backward compatibility behavior documented
- ✅ Updated API documentation (if applicable)
- ✅ Workflow authoring guide updated (in docs/)

## Out of Scope

- UI admin panel for reordering commands (potential future feature)
- Per-command visibility flags
- Frontmatter-based ordering (Option C) - not in MVP
- Numeric prefixes (Option B) - not in MVP
- Agent ordering (separate issue)
- Dynamic reordering via API (future enhancement)

## Testing Plan

### Unit Tests (Backend)

**Test Suite: `handlers/content.go` or separate test file**

```go
func TestParseCommandOrder_FileNotFound(t *testing.T) {
    // Case: No _order.yaml exists
    // Expected: nil error, nil order slice
}

func TestParseCommandOrder_ValidYAML(t *testing.T) {
    // Case: Valid _order.yaml with command list
    // Expected: Correct order slice returned
}

func TestParseCommandOrder_InvalidYAML(t *testing.T) {
    // Case: Malformed YAML file
    // Expected: Error returned, logged appropriately
}

func TestSortCommandsByOrder_FullOrder(t *testing.T) {
    // Case: All commands listed in order
    // Expected: Commands in exact specified order
}

func TestSortCommandsByOrder_PartialOrder(t *testing.T) {
    // Case: Only some commands in order
    // Expected: Specified first, rest alphabetical
}

func TestSortCommandsByOrder_MissingCommand(t *testing.T) {
    // Case: Order references non-existent command
    // Expected: Warning logged, command skipped, rest correct
}

func TestSortCommandsByOrder_Duplicates(t *testing.T) {
    // Case: Command listed twice
    // Expected: Deduplicated, first occurrence wins
}

func TestContentWorkflowMetadata_WithOrdering(t *testing.T) {
    // Case: Full integration with order file
    // Expected: Correct order applied end-to-end
}
```

### Integration Tests

**Setup**: Create temporary workflow directory structure with `.claude/commands/` and `_order.yaml`

```go
func TestWorkflowOrdering_Integration(t *testing.T) {
    // Create temp workflow with:
    // - command1.md, command2.md, command3.md
    // - _order.yaml listing command3, command1, command2
    // Call ContentWorkflowMetadata()
    // Verify order: command3, command1, command2
}

func TestBackwardCompatibility_NoOrderFile(t *testing.T) {
    // Create temp workflow WITHOUT _order.yaml
    // Call ContentWorkflowMetadata()
    // Verify order is alphabetical
}
```

### E2E Tests (Cypress)

**Test: Commands display in correct order**
```javascript
it('should display slash commands in specified order', () => {
  // 1. Create session with workflow containing _order.yaml
  // 2. Navigate to session page
  // 3. Open Workflows accordion
  // 4. Verify commands appear in order: plan, specify, analyze, clarify, tasks
  // 5. Verify order matches _order.yaml specification
})

it('should display commands alphabetically without order file', () => {
  // 1. Create session with workflow WITHOUT _order.yaml
  // 2. Navigate to session page
  // 3. Verify commands appear alphabetically
})
```

## Deployment & Rollout

### Deployment Steps

1. **Merge to main branch** with all tests passing
2. **Build backend container image** with new code
3. **Push to container registry** (quay.io/ambient_code)
4. **Deploy to UAT environment** via make deploy
5. **Smoke test** with UAT workflow containing `_order.yaml`
6. **Verify UI displays correct order** in Chrome browser

### Rollback Plan

If issues detected:
1. Revert commit
2. Rebuild container image
3. Re-deploy previous version
4. No data migration needed (order file is separate from CRs)

### Migration

**NO MIGRATION REQUIRED**
- Existing workflows without `_order.yaml` continue to work
- Order specification is completely optional
- No changes to CR structure or data storage

## References

- **Issue**: https://github.com/ambient-code/platform/issues/303
- **Requirements Doc**: See ISSUE_303_REQUIREMENTS.md
- **Backend Handler**: `components/backend/handlers/content.go:464-562`
- **Frontend Component**: `components/frontend/src/app/projects/[name]/sessions/[sessionName]/components/accordions/workflows-accordion.tsx`
- **Related Code**: `handlers/sessions.go::GetWorkflowMetadata()`

---

**Specification Version**: 1.0
**Status**: Ready for Implementation
**Last Updated**: 2025-11-13
