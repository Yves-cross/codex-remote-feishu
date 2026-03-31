---
name: ts-worker
description: Implements TypeScript features (server, bot, shared) with TDD and thorough testing
---

# TypeScript Worker

NOTE: Startup and cleanup are handled by `worker-base`. This skill defines the WORK PROCEDURE.

## When to Use This Skill

Features involving TypeScript components: relay server, Feishu bot, shared types. Covers REST API, WebSocket handling, session management, command parsing, message formatting, Feishu SDK integration.

## Required Skills

None.

## Work Procedure

1. **Read the feature description** carefully. Understand preconditions, expected behavior, and verification steps.

2. **Read architecture.md** at `.factory/library/architecture.md` for system context.

3. **Check shared types**: If the feature involves message types or protocol constants, check `shared/src/types.ts` first. Add new types there if needed.

4. **Write tests first (RED)**:
   - Create or update `.test.ts` files alongside source files
   - Use `vitest` with `vi.mock()` for mocking dependencies
   - For WebSocket tests: use `ws` library to create mock WebSocket servers/clients
   - For Feishu SDK tests: mock `@larksuiteoapi/node-sdk` — never make real API calls
   - For REST API tests: use `supertest` or direct `fetch` against the running server
   - Run `cd <package> && npx vitest run` — tests should FAIL (red)

5. **Implement (GREEN)**:
   - Write the minimum code to make tests pass
   - Use `zod` for runtime validation of incoming messages/payloads
   - Use proper TypeScript types — no `any`
   - Handle errors gracefully — return proper HTTP status codes, log errors with context

6. **Refactor**:
   - Clean up code, extract functions if needed
   - Run `cd <package> && npx vitest run` — all tests pass
   - Run `cd <package> && npx tsc --noEmit` — no type errors

7. **Verify manually**:
   - For server features: use `curl` to test REST API endpoints
   - For bot features: verify command parsing with unit tests (no real Feishu needed)
   - For integration: start server, connect with a WebSocket client, verify behavior

8. **Run full test suite for the package**: `cd <package> && npx vitest run`

## Example Handoff

```json
{
  "salientSummary": "Implemented session registry with online/offline/grace-period states and LRU eviction. Wrote 15 test cases covering registration, state transitions, grace period, reconnection, and eviction. All pass with `npx vitest run` (15 passing). Verified REST API with curl — GET /sessions returns correct session list.",
  "whatWasImplemented": "Added session.ts with SessionRegistry class. Supports register(), disconnect(), reconnect(), evict(). Sessions transition through online → grace-period → evicted states. Configurable grace period TTL and max sessions. LRU eviction when max exceeded. REST API endpoints GET /sessions and GET /sessions/:id wired up in api.ts.",
  "whatWasLeftUndone": "",
  "verification": {
    "commandsRun": [
      { "command": "cd server && npx vitest run", "exitCode": 0, "observation": "15 tests passed" },
      { "command": "cd server && npx tsc --noEmit", "exitCode": 0, "observation": "no type errors" },
      { "command": "curl http://localhost:9501/sessions", "exitCode": 0, "observation": "returns [] for empty registry" }
    ],
    "interactiveChecks": [
      { "action": "Started server, connected mock wrapper via wscat, sent register message", "observed": "GET /sessions now returns the registered session with state:idle, online:true" }
    ]
  },
  "tests": {
    "added": [
      {
        "file": "server/src/session.test.ts",
        "cases": [
          { "name": "registers a new session", "verifies": "register() adds session with idle state" },
          { "name": "rejects duplicate registration", "verifies": "second register with same ID returns error" },
          { "name": "transitions to grace period on disconnect", "verifies": "disconnect() sets online:false with grace timer" },
          { "name": "reconnect within grace period resumes session", "verifies": "reconnect() restores online:true and preserves state" },
          { "name": "evicts after grace period expires", "verifies": "session removed after TTL" }
        ]
      }
    ]
  },
  "discoveredIssues": []
}
```

## When to Return to Orchestrator

- Feature requires changes to the Rust wrapper protocol
- Feishu SDK behavior differs from documentation
- Shared type changes that would break other packages
- Integration test failures that indicate architectural issues
