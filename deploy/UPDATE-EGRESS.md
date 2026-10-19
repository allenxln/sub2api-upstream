# Managed image updates and private download egress

Custom images include pricing behavior that upstream binaries do not contain.
The Docker build marks them `managed`; the update service rejects binary updates,
backup rollback and upstream-version rollback before network or filesystem I/O.
The version menu continues checking upstream releases and directs administrators
to the pinned image release process. Official `release` builds keep their existing
binary update behavior. This is not an automatic upstream merge or release job.

`update-egress.py` provisions an independent Xray pair using trusted binaries staged on
both hosts and a supplied SHA256. It does not restart the existing proxy,
Sub2API, PostgreSQL, Redis or the host. Supply SSH destinations, public endpoint,
source IP, server name, the application's Docker bridge gateway and subnet explicitly.
Existing installations are refused rather than silently replacing credentials.

Each host's `/etc/sub2api-update-egress/config.json` is that endpoint's authority.
Credentials are generated privately, written over SSH stdin and never printed.
The server allows only the specified ECS source IP and GitHub HTTPS destinations.
The client listens on loopback and the private Docker bridge with HTTP Basic
Authentication. Neither a public HTTP proxy nor a public management API is added.
The independent server identity is revoked by stopping its dedicated service;
it is not a member of the shared personal proxy instance.

The `sub2api-update-egress.service` units start at boot and restart failed
processes. Access logging is disabled. Configs are root-owned, group-readable
only by the service account; the deployment environment file is root-only.

`/etc/sub2api-update-egress/updater.env` on the client contains the new
`UPDATE_PROXY_URL`. Provisioning deliberately does not change the application.
Adopt this value in the authoritative deployment configuration only during the
controlled image release, preserving the other environment values. Never print
or commit the resolved Compose file or the proxy URL containing its password.

Before adoption, verify unauthenticated rejection, full release download and its
official checksum from an isolated container on the application's network, denied
non-GitHub targets, and successful download after restarting the two egress units.
Verify app health and original proxy start time remain unchanged.

Rollback before application adoption: disable and stop only the two dedicated
units. Their stop hook removes the dedicated source-IP firewall rule. Remove
only the dedicated unit, config directory and binary directory after confirming
no application uses them. After application adoption, first restore its previous
proxy setting through the same controlled release process. Retain the original
pinned application image and never restore an old financial database over new
transactions.

A release receipt records the actual image and binary digests separately from
historical business capability attestations. Preserve exact existing business
proofs and frozen work-order payloads; a metadata or updater-only rebuild does
not justify rewriting their identities. Archive still-active proof material in
the live data/configuration directory, not an expiring backup directory, and
record its original provenance and hash. Verify window/billing code and migration
inputs against the deployed source before declaring capability continuity.
