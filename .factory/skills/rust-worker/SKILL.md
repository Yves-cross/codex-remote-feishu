---
name: rust-worker
description: Implements Rust wrapper features with TDD and thorough testing
---

# Rust Worker

NOTE: Startup and cleanup are handled by `worker-base`. This skill defines the WORK PROCEDURE.

## When to Use This Skill

Features involving the Rust wrapper binary: stdio proxy, message classification, WebSocket client, configuration, signal handling, child process management.

## Required Skills

None.

## Work Procedure

1. **Read the feature description** carefully. Understand preconditions, expected behavior, and verification steps.

2. **Read architecture.md** at `.factory/library/architecture.md` for system context.

3. **Write tests first (RED)**:
   - Create or update test files in the wrapper crate
   - For stdio proxy tests: use `tokio::io::duplex` to create mock stdin/stdout channels
   - For WebSocket tests: use a mock WebSocket server (e.g., `tokio-tungstenite` server on localhost)
   - For child process tests: create a simple mock binary (shell script or small Rust binary in `tests/fixtures/`)
   - Run `cd wrapper && cargo test` — tests should FAIL (red)

4. **Implement (GREEN)**:
   - Write the minimum code to make tests pass
   - Follow Rust conventions: no `unwrap()` in production, use `?` operator, proper error types
   - Use `tracing` for logging, never print sensitive data

5. **Refactor**:
   - Clean up code, extract functions if needed
   - Run `cd wrapper && cargo test` — all tests pass
   - Run `cd wrapper && cargo clippy` — no warnings

6. **Verify manually**:
   - If the feature involves stdio proxy: run the wrapper with a mock codex script and verify messages pass through
   - If the feature involves WebSocket: verify connection/registration with a simple WebSocket client

7. **Run full test suite**: `cd wrapper && cargo test`

## Example Handoff

```json
{
  "salientSummary": "Implemented JSONL message classifier that categorizes codex app-server messages into agentMessage, toolCall, serverRequest, and turnLifecycle types. Wrote 12 test cases covering all message types plus edge cases (malformed JSON, empty lines). All pass with `cargo test` (12 passing, 0 failing). Verified with manual test using mock codex script.",
  "whatWasImplemented": "Added classifier.rs with classify_message() function that parses JSONL and returns MessageType enum. Handles item/agentMessage/delta, item/started (commandExecution, fileChange, dynamicToolCall), serverRequest/*, turn/started, turn/completed. Unknown types return MessageType::Unknown.",
  "whatWasLeftUndone": "",
  "verification": {
    "commandsRun": [
      { "command": "cd wrapper && cargo test", "exitCode": 0, "observation": "12 tests passed, 0 failed" },
      { "command": "cd wrapper && cargo clippy", "exitCode": 0, "observation": "no warnings" }
    ],
    "interactiveChecks": [
      { "action": "Ran wrapper with mock codex that emits various message types", "observed": "agentMessage forwarded to mock server, toolCall not forwarded, all messages passed through to stdout" }
    ]
  },
  "tests": {
    "added": [
      {
        "file": "wrapper/src/classifier.rs",
        "cases": [
          { "name": "test_classify_agent_message_delta", "verifies": "item/agentMessage/delta classified as AgentMessage" },
          { "name": "test_classify_command_execution", "verifies": "commandExecution item classified as ToolCall" },
          { "name": "test_classify_server_request", "verifies": "serverRequest classified as ServerRequest" },
          { "name": "test_classify_turn_started", "verifies": "turn/started classified as TurnLifecycle" },
          { "name": "test_classify_malformed_json", "verifies": "invalid JSON returns Unknown without panic" }
        ]
      }
    ]
  },
  "discoveredIssues": []
}
```

## When to Return to Orchestrator

- Feature requires changes to the WebSocket protocol that affect server/bot
- Codex app-server protocol has undocumented behavior that blocks implementation
- Build failures in dependencies that can't be resolved
