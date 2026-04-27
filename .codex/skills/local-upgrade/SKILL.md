---
name: local-upgrade
description: "Use when the user asks for this repository's local upgrade flow: 本地升级, 拉最新代码后升级本地 daemon, upgrade-local.sh, or triggering the built-in local upgrade transaction from a repo build. Prefer the repo helper script and fixed local-upgrade artifact path; do not use the removed install -upgrade-source-binary flag."
---

# local-upgrade

Use this skill when the task is to refresh the locally installed `codex-remote` daemon from the current repository checkout.

## Default path

Prefer this command from the repo root:

```bash
./upgrade-local.sh
```

That script does all of the following:

1. `git pull --ff-only`
2. rebuild `./bin/codex-remote`
3. copy the new binary to the fixed local artifact path
4. run `./bin/codex-remote local-upgrade`

For upgrading the daemon instance that is currently hosting the active Codex conversation, prefer:

```bash
./upgrade-self.sh
```

## Natural-Language Boundary

- Natural-language `本地升级` requests are repository tasks, not daemon slash-command requests.
- For repo-build local upgrade requests:
  - use `./upgrade-local.sh`
  - when the user names a target instance, keep that target explicit with `--instance <id>`
  - do **not** send `/upgrade ...` back into whichever daemon is currently hosting the Codex conversation
- For self-recovery requests where the current daemon is too old or too broken to rely on its own `/upgrade dev` or `upgrade local` entrypoints:
  - use `./upgrade-self.sh`
  - this path builds a fresh repo binary first, then uses that fresh binary to drive `local-upgrade` against the current daemon self target
- Natural-language debug/status/log/bug requests such as `debug 一下`, `看下当前实例状态`, `查日志`, or `报个 bug` default to the current daemon **self target**, not the repo-bound target.
- Only use `bash scripts/install/repo-install-target.sh --format shell` or `bash scripts/install/repo-target-request.sh ...` when the user explicitly asks for:
  - the repo-bound target
  - or a named install target such as `stable` / `beta` / `master`
- Explicit slash commands such as `/upgrade`, `/upgrade local`, `/upgrade latest`, and `/debug` remain direct daemon actions on the daemon that received that slash command.

## Variants

- Different install base dir:

```bash
./upgrade-local.sh --base-dir /path/to/base
```

- Explicit target instance:

```bash
./upgrade-local.sh --instance beta
```

- Upgrade the daemon that is currently hosting this Codex session:

```bash
./upgrade-self.sh
```

- Explicit slot label:

```bash
./upgrade-local.sh --slot local-test
```

- Dirty worktree:
  - default behavior is to stop before `git pull`
  - only use `--allow-dirty` when the user explicitly wants to keep going despite local changes

## Notes

- The built-in CLI entry is `codex-remote local-upgrade`.
- The fixed artifact path is `~/.local/share/codex-remote/local-upgrade/codex-remote` for the default base dir.
- For explanation or debugging of the current self-upgrade transaction, prefer `docs/general/local-self-upgrade-flow.md` before re-reading install code.
- If the script says `install-state.json` is missing, bootstrap the local install first with `./setup.sh` or point `--base-dir` at the installed environment.
- For explicit repo-bound or named-target debug/status HTTP calls, prefer:

```bash
bash scripts/install/repo-target-request.sh admin /v1/status | jq .
```

```bash
bash scripts/install/repo-target-request.sh admin /api/admin/bootstrap-state | jq .
```

```bash
bash scripts/install/repo-target-request.sh --instance beta admin /v1/status | jq .
```

- For explicit current self-target debug/status HTTP calls, prefer:

```bash
bash scripts/install/self-target-request.sh admin /v1/status | jq .
```

- For explanation-only requests, `./upgrade-local.sh --help` is usually enough.
