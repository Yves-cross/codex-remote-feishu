---
name: safe-push
description: "Use when pushing committed changes from this repository, especially when `git push` may fail because the remote branch advanced. Prefer the repo helper script that fetches, rebases if needed, reruns tests after a successful rebase, and only then pushes."
---

# safe-push

Use this skill when the task is to push already-committed local changes from this repository.

Prefer this command from the repo root:

```bash
./safe-push.sh
```

## Default behavior

The helper only automates the safe happy path:

1. require a clean worktree
2. `git fetch` the target branch
3. if the remote branch moved ahead, `git rebase` onto it
4. rerun `go test ./...` only when a rebase actually happened
5. push only if all previous steps succeed

## Important limits

- It does not auto-resolve rebase conflicts.
- It does not auto-handle test failures.
- On conflict or test failure, it stops and leaves the repo state visible for manual handling.

## Useful variants

- Push a non-default branch:

```bash
./safe-push.sh --branch feature-x
```

- Use a narrower post-rebase test command:

```bash
./safe-push.sh --test-cmd 'go test ./internal/adapter/feishu ./internal/core/orchestrator ./internal/app/daemon'
```

- Force tests even when no rebase happened:

```bash
./safe-push.sh --always-test
```

- Skip tests entirely:

```bash
./safe-push.sh --no-test
```
