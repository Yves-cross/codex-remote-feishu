# AGENTS

## Proxy Environment

This repository is often developed on hosts where `http_proxy` / `https_proxy` are set globally.
Those variables frequently interfere with local testing, especially for:

- `curl http://127.0.0.1:...`
- local health checks
- websocket/http calls to local relay services
- integration tests that expect direct localhost access

Before running local tests or local debugging commands, clear proxy-related environment variables in the shell used for the test:

```bash
unset http_proxy https_proxy HTTP_PROXY HTTPS_PROXY ALL_PROXY all_proxy
```

Recommended rule:

- Default for local testing/debugging: proxy env must be unset.
- Default for localhost requests: proxy env must be unset.

## Wrapper Exception

There is one important exception:

- `relay-wrapper` itself should run without inheriting proxy env for its own local relay communication.
- But when `relay-wrapper` launches the real `codex` binary (`codex.real`), it must restore the captured proxy env for the child process.

Reason:

- local wrapper <-> relayd / localhost traffic is easily broken by proxy interception
- upstream `codex.real` <-> ChatGPT/OpenAI traffic is more stable when it uses the configured proxy

So the intended behavior is:

1. wrapper process starts and clears proxy env for itself
2. wrapper communicates with local relay services without proxy
3. wrapper spawns `codex.real` with the previously captured proxy env restored

Any future changes to startup, testing scripts, or process launching must preserve this rule.

## Stateful Debugging Rule

For bugs that involve multiple layers or state machines (for example VS Code <-> wrapper <-> relayd <-> Feishu):

- Do not patch the first plausible cause and stop.
- First collect runtime evidence from the full path: current server state, relevant logs, and the actual event/control flow.
- Translate the user-reported reproduction into tests before or together with the fix.
- If multiple layers participate in the bug, fix the whole chain in one pass instead of doing isolated partial tweaks.
- Do not consider the issue fixed just because unit tests pass; verify that the observed runtime state actually changes in the expected way.

This rule exists because partial fixes on stateful flows often leave the visible behavior unchanged and waste debugging cycles.
