# Windows delivery proposal (not enabled)

Recommended route for the personal MAX bridge: hosted CI builds a committed revision;
publish an immutable release only after checks; Windows pulls a verified artifact,
stages it beside the current binary, and activates it only at an observable idle gate.
Keep bot tokens, Codex authentication and local configuration on Windows.

Bind the artifact to repository, branch/tag, full source commit, architecture and
digest. Prefer GitHub artifact provenance attestations. Use minimal workflow
permissions and pin workflow dependencies. A checksum downloaded alongside a binary
is useful for corruption detection but does not independently prove publisher identity.

Reuse the existing Task Scheduler daemon as runtime owner. A deployment operation
needs one lock/writer, a timeout, retained previous binary/configuration, startup
readback and automatic rollback on failure. Never replace the running binary in place.
An idle gate must include running turns, queued messages and pending questions;
process existence or low CPU is not proof of readiness to update.

Current repository CI runs on main pushes and pull requests. No automatic release
or Windows pull-updater was found. Broad Windows-suite exceptions used for local
commits are not an automatic production-release approval. Publish/activation scope
still needs the user's choice; nothing has been pushed or enabled by this proposal.

The existing `cmd/cc-connect/update.go` is the update owner. It currently hardcodes
`chenhg5/cc-connect` and also uses an upstream Gitee fallback. Extend this owner with
an explicit fork/release-source configuration and verification policy before enabling
automated delivery; accepting an upstream release would overwrite fork-specific fixes.

Avoid attaching a general self-hosted Actions runner to this personal Windows PC.
GitHub recommends against such runners for public repositories. GitHub-hosted builds
plus a narrowly scoped local pull step keep repository workflows away from bot secrets.

Primary references checked 2026-11-16:
- https://docs.github.com/en/actions/how-tos/secure-your-work/use-artifact-attestations
- https://docs.github.com/en/code-security/tutorials/implement-supply-chain-best-practices/securing-builds
- https://docs.github.com/en/actions/reference/security/secure-use
- https://learn.microsoft.com/en-us/windows/win32/taskschd/task-scheduler-start-page
