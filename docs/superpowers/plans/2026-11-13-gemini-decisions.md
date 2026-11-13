# Gemini Decisions Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make Google's native Decisions API available through Sub2API with correct routing, proxy use and accounting for Bigpot.

**Architecture:** Extend the existing Gemini request lifecycle with a Decisions parser and a separate upstream transport/codec. Native configuration remains the owner of accounts, group access, proxies and prices.

**Tech Stack:** The production Sub2API Go 1.27.3 stack and its existing test tools; existing native UI/configuration.

**Spec:** `docs/superpowers/specs/2026-11-13-gemini-decisions.md`

## Global constraints

- Start from the verified production commit `10ba8f9b4dc2e6023d4a9879ab77b21841babd10`.
- Use the task worktree, shared compatible Go caches and `/tmp/dot-heavy-build.lock` for heavy compilation; limit Go parallelism to two.
- Keep normal Gemini and Jev behavior, native account policy and accounting ownership.
- Never log or commit credentials, real user inputs or production exports.
- Production configuration uses the official admin API, timestamped before-images and exact readback diffs. No direct ledger/database writes.

## Review focus

- Google usage differs from generative Gemini usage: exact input tokens, zero output and correct CNY amount.
- Multimodal/extension fields and answer probabilities must survive without generative-content rewriting.
- Composite group aliases and allowlists must examine the public body model before rewriting.
- Proxy errors, 429 and malformed caller requests must respect native failover/account-state boundaries.
- Moderation and model metadata must recognize the new protocol rather than silently treating it as a chat model.

### Task 1: Decisions protocol and transport

**Files:** Add `backend/internal/pkg/gemini/decisions.go` and tests; add `backend/internal/service/gemini_decisions.go` and tests; extend Gemini model metadata and the existing native-forward dispatch.

**Interfaces:** The pure codec validates the routing envelope and decodes authoritative usage. The existing `GeminiMessagesCompatService.ForwardNative` handles the Decisions operation using the same proxy/error policy, a separate Decisions request builder and bounded response adapter, returning existing `ForwardResult` metadata.

- [x] Write and run failing tests for a real local HTTP upstream: text and multimodal passthrough, model mapping, API-key authentication, native proxy selection, exact usage and caller/upstream error behavior.
- [x] Implement the codec and transport using existing URL validation, HTTP transport and error-policy helpers.
- [x] Verify the package/service tests and preserve evidence; commit the tested protocol implementation.

### Task 2: Native gateway integration

**Files:** Extend `backend/internal/handler/gemini_v1beta_handler.go`; add a Decisions request parser; update `backend/internal/server/routes/gateway.go`, endpoint metadata and moderation extraction; add focused tests alongside these owners. Extend native account testing only where required to test this protocol correctly.

**Interfaces:** `GatewayHandler.GeminiV1BetaDecisions` consumes native authentication/group context and the body model, then uses the same admission, scheduling and billing lifecycle as other Gemini requests.

- [ ] Write and run failing tests for body-model admission/routing, protocol rejection, moderation and Decisions model metadata.
- [ ] Add the native route and parser; reuse the existing request lifecycle without duplicating accounting or retry loops.
- [ ] Verify normal Gemini/Jev regression tests and the project checks appropriate to this backend-only change; reconcile a literal billing fixture; review the complete diff and commit.

### Task 3: Candidate acceptance and delivery

**Files:** Add a safe verification receipt under `docs/verification/`; keep secrets and private before-images outside Git.

- [ ] Verify Google Decisions from the production egress through `bestapi-egress`, using the approved free project and synthetic input.
- [ ] Establish the current writer, release lock, resource budget and safe candidate/switch mechanism before production writes.
- [ ] Prepare native account/model/price changes and their exact before/after diff; verify a candidate gateway path and its usage/billing record before activation.
- [ ] Complete one fresh whole-branch review, integrate through the repository's review rules, push and verify the owned remote `main` contains the task commits.
- [ ] Activate only after release gates pass; verify the actual member-facing gateway path, native records and unchanged existing protocol behavior. Report any concrete blocker without claiming activation.
- [ ] Clean task-owned temporary credentials, build intermediates and merged feature worktree/branches; retain evidence and required rollback material.
