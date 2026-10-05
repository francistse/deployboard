## What this changes

<!-- One or two sentences. The "why", not a restatement of the diff. -->

## How it was verified

<!-- Real evidence: the command you ran and its output, a test that now passes,
     a curl against a live instance, a DOM measurement. "Tested locally" is not evidence. -->

## Checklist

- [ ] `make vet && make test` pass
- [ ] `npm test` passes (and `landing`: `npm run lint && npm run build`) if frontend/landing touched
- [ ] No new `github.com/A404coder/launch-pilot/...` imports (module path is `.../deployboard`)
- [ ] Invariants intact: 7 statuses, two read-only levels, plists are never edited
- [ ] Upstream compatibility considered (see `docs/UPSTREAM.md`) — nothing upstream is removed
