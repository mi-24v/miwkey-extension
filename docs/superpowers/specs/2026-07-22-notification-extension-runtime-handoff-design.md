# miwkey-extension Runtime Handoff Design

- Date: 2026-07-22
- Status: Approved for implementation in a separate session
- Repository: `miwkey-extension`

TLS clarification (2026-11-19): the original HTTP-only assumptions in sections
5 and 9 are superseded for the TLS runtime by
[section 3.3 of the AWS integration design](https://github.com/mi-24v/miwkey-aws/blob/ac6654b8c12e650f94067d211da5809a962557e5/docs/specs/notification-extension-aws-integration-design.md#L89).
That section defines the TLS certificate, startup-log and health-check contract;
[section 3.4](https://github.com/mi-24v/miwkey-aws/blob/ac6654b8c12e650f94067d211da5809a962557e5/docs/specs/notification-extension-aws-integration-design.md#L100)
defines the dedicated Misskey client. Both references are pinned to the published
commit `ac6654b8c12e650f94067d211da5809a962557e5`. The original date and status
above describe the handoff design, not a new approval of this clarification.
The other responsibilities and verification requirements remain in scope.

## 1. Goal

Prepare the extension server and the embedded Misskey fork for the internal AWS
interface defined in `miwkey-aws`.

This document contains only work owned by `miwkey-extension`. ECS, DynamoDB,
Secrets Manager, Service Connect, SSM, ASG, and deployment orchestration belong
to the AWS repository design.

## 2. Scope

This design covers:

- coordinating the Misskey fork change that reads the notification extension
  URL and secret from environment variables
- advancing the `misskey` gitlink after that fork change is complete
- publishing the extension server image from this repository
- multi-architecture support for the extension server image
- extension health checks
- the extension logging contract
- local and integration-test compatibility

This design does not cover:

- building or publishing the Misskey application image
- changing the Misskey application image tag in ECS
- Misskey database migrations or migration concurrency control
- AWS resources or IAM policies
- production notification import execution

The Misskey fork repository owns its own application image build and release.
The `miwkey-extension` CI builds only the extension server image.

## 3. Misskey Configuration Interface

Implement this change in the Misskey fork repository, then update the `misskey`
gitlink in `miwkey-extension`. Notification extension configuration can come
from:

- `NOTIFICATION_EXTENSION_URL`
- `NOTIFICATION_EXTENSION_SECRET`

Environment variables take precedence over YAML values. YAML remains a
fallback for local development. Enable the integration only when both URL and
secret are present. If only one is present, keep the integration disabled and
emit a configuration warning without printing either value.

Preserve the existing optional timeout behavior.

Add unit tests for:

- environment-only configuration
- YAML-only fallback
- environment variables overriding YAML
- neither source configured
- only URL configured
- only secret configured

## 4. Extension Image Publication

Publish only the extension server image from this repository:

```text
ghcr.io/mi-24v/miwkey-extension
```

The production release flow is:

- merge to `main`
- run the extension Go tests
- build and push a public GHCR package
- publish `linux/amd64` and `linux/arm64` in one manifest
- publish the immutable `sha-<commit>` tag
- optionally publish `latest`, while production ECS continues to use the SHA
  tag

The repository must have a real `main` branch and use it as the protected
production merge target. The current Docker workflow already names `main`, but
the branch must exist and the workflow must be verified after branch setup.

Use Buildx with both target platforms. Add QEMU setup if the selected GitHub
runner requires emulation for the ARM build.

Do not add a Misskey application image build to this workflow. That image is
built and published by the Misskey fork repository.

## 5. Health Check

Add an unauthenticated `GET /healthz` endpoint that returns only service health
and no application data. It must not require a JWT.

The runtime image is distroless, so do not depend on `curl`, `wget`, or a shell
for the container health check. Add a health-check mode to the extension binary
that requests `127.0.0.1:8080/healthz` and exits nonzero on failure. TLS is the
default: connect over HTTPS with SNI `TLS_SERVER_NAME` and verify the certificate
against the system roots, including its name and validity period. Do not fetch
the certificate secret in health-check mode. Plain HTTP is only for explicit
local-development mode (`TLS_MODE=http`).

The health endpoint must not inspect or mutate notification data. Its purpose is
to confirm that the process is ready to accept requests; it returns 204 without
checking the database.

## 6. Logging Contract

Write application logs to stdout/stderr. The application logs may contain:

- Echo request metadata
- startup and graceful shutdown events
- panic/recover output
- DynamoDB and AWS configuration errors without payload data

Do not log:

- notification payloads
- JWTs
- Authorization headers
- the shared secret
- DynamoDB item contents

The AWS layer sends these stdout/stderr records to the application log group.
Service Connect proxy logs and access-log settings are owned by `miwkey-aws`
and are not configured in this repository.

## 7. DynamoDB Local Compatibility

Keep the existing `DYNAMODB_AUTO_CREATE` behavior for local use and integration
tests. `DYNAMODB_AUTO_CREATE=true` may create the DynamoDB Local table and GSI.

Production leaves this variable unset because the AWS deployment creates the
table. Do not remove the local auto-create feature and do not expand it into a
production migration system.

## 8. Verification

- Go tests pass.
- Misskey configuration precedence tests pass.
- An incomplete URL/secret pair does not enable the integration.
- The health check fails when `/healthz` is unavailable.
- The health check succeeds when the service is ready.
- The GHCR manifest contains amd64 and arm64 images under the same SHA tag.
- The GHCR package is public.
- The integration test using DynamoDB Local still passes with auto-create.
- Logs and test output contain no payloads, JWTs, Authorization headers, or
  secret values.

## 9. External Contract

The AWS deployment provides:

- `AUTH_SECRET` to the extension container
- `DYNAMODB_NOTIFICATION_TABLE` to the extension container
- `TLS_CERT_SECRET_ARN` and `TLS_SERVER_NAME` to the extension container
- `NOTIFICATION_EXTENSION_URL` to the Misskey container
- `NOTIFICATION_EXTENSION_SECRET` to the Misskey container
- an internal Service Connect endpoint on port 8080 whose `clientAliases.dnsName`
  is the certificate SAN FQDN, also used as `TLS_SERVER_NAME`

The extension server continues to listen on port 8080 with TLS 1.2 or later by
default. Missing TLS settings, invalid mode values or certificate validation
failures stop startup without falling back to HTTP. Explicit `TLS_MODE=http`
is only for local development.

The certificate is loaded from Secrets Manager's explicit `AWSCURRENT` version
at startup with a bounded timeout. The JSON fields are `certificatePem`
(full chain) and `privateKeyPem`; both stay in memory. Verify the key pair,
SAN, validity period and chain to system roots before serving. Emit one
non-secret JSON certificate record after successful TLS configuration and
listener bind, as specified in the AWS design section 3.3.

The Misskey fork uses `https://<extension-fqdn>:8080` in production. The URL
hostname must match the certificate SAN FQDN and `TLS_SERVER_NAME`; the short
service name `miwkey-extension` is not a substitute for that hostname. It signs
requests with short-lived HS256 JWTs using the shared secret. Keep the dedicated client
from `f68926918fb8950d4a642ed564bed189a6740f19` together with the Node 22.23.4
security update from `a645fc53522177b5b7c43610fa11cbc82cb0bdd8` when resolving
the gitlink. Pin a commit containing both histories: either previous tip alone
loses a required change. HTTP origins remain a local fallback only.
