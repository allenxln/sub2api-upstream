# Gemini Decisions verification receipt

The implementation extends the native Gemini lifecycle from the deployed
Sub2API 0.2.21 baseline `10ba8f9b4dc2e6023d4a9879ab77b21841babd10`.
The Go build patch is upgraded to 1.27.4 for the reachable net/http security
fix GO-2026-6859. No schema, separate Bigpot UI, Jev protocol, generic retry or ledger change is
included. Native accounts, proxies, groups and channels remain configuration
owners.

## Source verification

- Complete `go test -tags unit -p 2 ./... -count=1`: PASS after the independent
  review fixes. A second complete run with Go 1.27.4 also passed.
- Native frontend build: PASS using the committed lockfile, Node 22.23.2 and
  pnpm 9.15.9. Frontend source and lockfile are unchanged.
- `git diff --check`: PASS.
- GitHub backend tests, golangci-lint, frontend build, frontend security,
  backend security and release helpers: PASS on `bb41b8d05`. The macOS shell
  jobs were cancelled; the exact six shell checks passed locally on macOS.
- Absolute billing fixture: 174 input tokens at CNY 0.65/M gives CNY 0.00011310
  before the group factor; factor 0.3 gives CNY 0.00003393. Output/cache fees
  are zero. The provider paid-reference fallback is USD 0.10/M input.

One fresh independent review found three functional issues. Reproduction tests
failed before the fixes and passed afterward: initial/failover selection keeps
a question named `thoughtSignature` and the exact number 9007199254740993;
OAuth/service-account candidates cannot intercept a Decisions request; model
alias list/get discovery retains mapped Decisions capabilities. Existing Gemini
model discovery and wildcard allowlist tests also passed.

## Upstream and release acceptance

A synthetic request from the production host through its existing configured
proxy returned Google HTTP 200, with authoritative usage of 101 input tokens
and zero output. A direct connection had timed out. This proves provider and
proxy compatibility. The isolated native container subsequently returned 200
with 174 input tokens and zero output, with its native bill, wallet and key
counter reconciled to the literal amounts above.

Production activation completed on 2026-11-14 (Asia/Shanghai). The deployed
artifact is `0.2.21-gemini-decisions.2`, built with Go 1.27.4 from
`7452e82aa0bafbbce866945ef209831ae897f853`, SHA-256
`7a3a9e9bb41c697955939b2eae4e6dc0c59e3bf6b411b54a01e471e33c355a44`.
Subsequent CI/docs commits do not change the runtime source. Its executable is
pinned by a read-only bind mount so the stock self-updater cannot replace the
custom integration silently. The original binary and configuration backups
remain available for rollback.

A blue/green switch rehearsal verified that existing connections remain on
their original instance while fresh Nginx and direct Bigpot connections select
the new instance, and that rollback restores fresh connections. Before stopping
each old instance, ingress connections, account slots and in-flight financial
reservations remained zero for at least 30 seconds. The original native
environment values and published ports were preserved. The temporary candidate
was retired, all three temporary NAT rules were removed, and the Nginx site was
restored byte-for-byte. Only the native application container was recreated.

Native account 169 now binds only to the production composite group 35 and the
existing proxy 1. Channel 29 gained one CNY 0.65/M input, zero-output row;
an exact Gemini composite route was added, followed by the allowlist publication.
Readback verified all unrelated configuration and the monthly limit 800,
unlimited daily/weekly limits and default factor 0.3.

A disposable key was cached before publication: the same company-domain request
returned 404 before publication and 200 afterward, without key rotation. The
production request returned 174 input/0 output tokens; native input/total cost
was CNY 0.0001131000, actual cost CNY 0.0000339300, wallet balance 0.99996607
from 1.00000000, and key usage 0.00003393. Output/cache charges were exactly zero.
Both native endpoints were `/v1beta/decisions`. Bearer authentication with list
and single-model discovery advertised only Decisions; streaming and
`generateContent` were rejected before upstream generation.

All disposable users and keys and the isolated group/channel were deleted.
The read-only consistency check retained its four pre-existing findings and
introduced none. Normal Gemini/Jev regressions passed in the complete backend
suite; no paid live Jev generation was made during this release. Private
configuration receipts and credentials remain outside Git.

The Go patch release is verified against the [official release history](https://go.dev/doc/devel/release#go1.27.4). The Docker build version is checked
against backend/go.mod by the security workflow.

The free tier may use input/output to improve Google products. Consult the
[current pricing](https://aistudio.google.com/docs/pricing#gemini-decision) and
[protocol documentation](https://aistudio.google.com/docs/decisions-api).
