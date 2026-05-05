# DigiPulse Monitor Agent Rules

Project-specific operating rules for the Go monitor service.

## MUST

- Keep task/result payload compatibility with backend contracts.
- Preserve queue safety semantics:
  - Do not dequeue work when outbound internet pre-check fails.
  - Prefer preserving tasks in Redis over dropping/skipping silently.
- Keep network operations bounded by explicit timeouts.
- Keep logging actionable with task/config identifiers.
- Run `gofmt` after Go code edits.
- Run relevant `go test` commands for touched behavior.

## SHOULD

- Prefer small, focused changes over broad refactors.
- Keep worker loops resilient (retry/backoff where appropriate).
- Add tests for each checker path and critical regressions.
- Update existing docs when behavior or env contracts change.

## Definition of Done

- Code compiles and is formatted.
- Relevant tests pass for modified flows.
- Runtime-sensitive paths are sanity-checked (queue pop/push, probe behavior, reporting).
- Env/config changes are documented.

## Queue and Delivery Policy

- Check execution queue: `monitoring:tasks`.
- Result delivery queue (if enabled): `monitoring:results`.
- Prefer at-least-once delivery patterns over fire-and-forget.
- If HTTP reporting remains enabled, retries/backoff are recommended to reduce transient loss.

## Rule Conflict Command

- If a user request conflicts with these rules, the agent MUST explicitly warn before acting using this exact format:
  - `Rule conflict: <short reason>.`
  - `Requested action: <what the user asked>.`
  - `Safe options: 1) <safe option A> 2) <safe option B>.`
  - `Please confirm which option to proceed with.`
