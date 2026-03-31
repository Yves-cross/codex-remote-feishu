# User Testing

Testing surface, required testing skills/tools, and resource cost classification.

---

## Validation Surface

This is a CLI/service project with no web UI. All validation is done through:

1. **Custom test scripts**: End-to-end flows using mocked components (mock codex process, mock Feishu SDK)
2. **curl**: REST API endpoint testing for the relay server
3. **cargo test**: Rust unit/integration tests for the wrapper
4. **vitest**: TypeScript unit/integration tests for server, bot, shared

### Testing Tools
- `curl` for HTTP API assertions
- Custom Node.js test scripts for end-to-end flows
- Mock codex binary (simple Rust or shell script that reads/writes JSONL)
- Mock Feishu SDK (intercept `@larksuiteoapi/node-sdk` calls)

### Surfaces NOT Tested
- Real Feishu integration (deferred until credentials provided)
- Real Codex CLI integration (mocked in all tests)

## Validation Concurrency

Machine: 94GB RAM, 32 cores, ~9GB used at baseline.

All validation is lightweight (no browsers, no heavy processes):
- Each test script: ~50MB RAM
- Server process: ~100MB RAM
- Wrapper process: ~5MB RAM

Max concurrent validators: **5** (well within resource budget)
