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
proxy compatibility, not activation of the member gateway.

An isolated native acceptance group, CNY price card and approved free-tier
Google account have been prepared and read back. The production member group
was not changed at this stage. Native gateway usage rows, actual wallet/key
debits, controlled publication and connection draining remain release gates.
No live activation is claimed by this source receipt. Private configuration
receipts and credentials are excluded from Git.

The Go patch release is verified against the [official release history](https://go.dev/doc/devel/release#go1.27.4). The Docker build version is checked
against backend/go.mod by the security workflow.

The free tier may use input/output to improve Google products. Consult the
[current pricing](https://aistudio.google.com/docs/pricing#gemini-decision) and
[protocol documentation](https://aistudio.google.com/docs/decisions-api).
