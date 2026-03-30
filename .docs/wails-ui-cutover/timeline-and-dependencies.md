# Wails Cutover — Timeline & Dependency Map

> Planning artifact for execution sequencing, not a hard deadline contract.

## Estimated Effort (Single Contributor)

1. Wave 1 (bootstrap): 0.5 - 1.0 day
2. Wave 2 (uiapi refactor): 1.0 - 1.5 day
3. Wave 3 (frontend shell tabs): 1.5 - 2.5 days
4. Wave 4 (data/state wiring): 1.0 - 1.5 day
5. Wave 5 (build/installer integration): 0.5 - 1.0 day
6. Wave 6 (validation/cutover): 1.0 - 1.5 day

Total expected: 5.5 - 9.0 working days.

## Critical Path

`Wave 1 -> Wave 2 -> Wave 3 -> Wave 4 -> Wave 5 -> Wave 6`

Notes:
- Wave 3 cannot start safely before Wave 2 because frontend bindings would otherwise lock in unstable APIs.
- Wave 5 should wait until Wave 3+4 pass basic usability checks to avoid packaging churn.

## Parallelizable Items

- While Wave 3 is in progress, draft Wave 6 smoke script skeleton in parallel.
- While Wave 4 stabilizes, prepare installer spec updates for Wave 5.

## Risk-Weighted Checkpoints

Checkpoint A (after Wave 2):
- Decide whether command parity is stable enough to proceed.
- If not stable, pause frontend build and resolve parity first.

Checkpoint B (after Wave 4):
- Confirm UX responsiveness/reconnect behavior.
- If unstable, keep harness as default and continue hardening.

Checkpoint C (after Wave 6):
- Execute final go/no-go for switching default UI runtime.
