# Gemini Decisions native gateway support

## Approved outcome

Allen approved adding the Google Decisions endpoint, upstream forwarding,
usage/billing integration and tests, and requested verification of the existing
IP proxy. Bigpot members should use `gemini-decision-1` through their existing
gateway account and see the correct model and consumption in native records.

## Source and ownership

The production binary reports Sub2API 0.2.21, commit
`10ba8f9b4dc2e6023d4a9879ab77b21841babd10`; image labels are older.
Implementation starts from that exact commit in a separate worktree. The
reference submodule checkout and unrelated tasks remain intact. The upstream
Go/Vue stack is covered by the explicit Sub2API exemption.

Google owns request semantics, inference and project quotas. Sub2API owns
authentication, group/model admission, account selection, proxies, failover,
concurrency, billing and usage. Bigpot's existing native configuration and
人民币核价 remain the source of customer-facing configuration; no parallel
model registry, quota scheduler or ledger is introduced.

## Approach

Add native `POST /v1beta/decisions` under the Gemini platform and reuse the
existing Gemini request lifecycle. Add a separate Decisions transport/codec so
generative-content transformations cannot alter decision inputs or answers.
Keep Google's response body and map its authoritative input-token usage into
the existing billing record; output, thinking and cache charges are zero.
Model discovery must advertise the Decisions method rather than text generation.
Existing moderation must inspect decision inputs and question text.

A configuration-only Jev mapping is unsuitable: it cannot translate the
Google protocol, and the native System One path identifies/bills Jev. A
standalone proxy with its own authentication or accounting would duplicate
native owners. The native implementation preserves provider semantics and
allows subsequent decision models to use the same endpoint.

## Proxy and price

Production direct TCP connection to Google timed out. The existing authenticated
`bestapi-egress` proxy completed Google TLS and received an HTTP response.
Configure the new Google account to use that native proxy. Keep TLS verification
and use the existing transport/error policy; do not add a separate proxy tool.

The existing Bigpot contract uses corresponding paid reference prices for free
sources. Google's standard input price is USD 0.10 per million tokens and output
is zero; the established fixed conversion gives CNY 0.65 per million input
tokens. Group discounts remain separate and must be read from current native
configuration before a concrete price change. For example, 174 input tokens at
base CNY 0.65/M and group multiplier 0.3 cost CNY 0.00003393.

## Acceptance and release boundary

Use synthetic fixtures for protocol, multimodal preservation, aliases,
admission, moderation, transport, account-error policy and billing tests.
Exercise the existing proxy from the actual production egress with a small
synthetic Google request. A gateway request must identify the new model and
produce a native usage row whose absolute token counts and CNY amount reconcile.

Source publication, candidate acceptance and production activation are separate
milestones. A gateway binary change is a topology-level release: preserve the
running binary/assets, verify release ownership and locks, validate a candidate,
protect in-flight requests, and stop before any switch whose safe mechanism
cannot be established. Never use live users for validation or restore a stale
database over new transactions. Any unresolved release condition is reported
with the code delivery and evidence.

References: https://aistudio.google.com/docs/decisions-api,
https://aistudio.google.com/docs/pricing#gemini-decision,
https://aistudio.google.com/docs/rate-limits#gemini-decision.
