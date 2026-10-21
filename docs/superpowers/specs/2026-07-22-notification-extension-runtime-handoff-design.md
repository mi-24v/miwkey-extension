# miwkey-extension Runtime Handoff Design

- Date: 2026-07-22
- Status: Approved for implementation in a separate session
- Repository: `miwkey-extension`

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
or another self-contained mechanism that performs an HTTP request to
`127.0.0.1:8080/healthz` and exits nonzero on failure.

The health endpoint must not inspect or mutate notification data. Its purpose is
to confirm that the process is ready to accept HTTP requests.

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
- `NOTIFICATION_EXTENSION_URL` to the Misskey container
- `NOTIFICATION_EXTENSION_SECRET` to the Misskey container
- an internal Service Connect endpoint named `miwkey-extension` on port 8080

The extension server continues to listen on port 8080. The Misskey fork treats
the URL as an internal HTTP endpoint and signs requests with short-lived HS256
JWTs using the shared secret.
