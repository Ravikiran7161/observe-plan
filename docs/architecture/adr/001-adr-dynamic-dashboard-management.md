---
adr-number: 001
owners:
  - jozef-reisinger
owners-verified-date: 2026-04-15
next-review-date: 2026-07-15
contributors:
  -
status: accepted

---

# Dynamic Dashboard Management ADR

## Context

User Story `910422` required dashboards that are easy to change and are created,
updated, and deleted automatically.

The main design choice was whether to manage dashboards as static assets in a
pipeline or to reconcile them dynamically in code.

## Related ADO Work Items

- **Feature(s):**
  - [Feature 906687: SaaS Observability Plane: Technical Logs, Metrics and Alerting](https://dev.azure.com/cmd-sw/dd842fe3-f288-46de-934e-b7a7aa42b2e1/_workitems/edit/906687)
- **User Story (if relevant):**
  - [User Story 910422: Dashboards for Metrics](https://dev.azure.com/cmd-sw/dd842fe3-f288-46de-934e-b7a7aa42b2e1/_workitems/edit/910422)

## Decision

Use a Lambda-managed dynamic CloudWatch dashboard architecture.

The chosen pattern is:

1. Provision the dashboard manager with CDK.
2. Run it during stack create, update, and delete through a CloudFormation
   custom resource.
3. Refresh the dashboard later with EventBridge.
4. Discover resources by tags and generate widgets in code.

This was chosen because the dashboard scope is dynamic and the story requires
automatic create, update, and delete behavior.

## Analysis

The key design tradeoff was static vs. dynamic dashboard management. Static approaches (pipeline-managed or CDK-defined) cannot track dynamically tagged resources without manual updates. A Lambda-based custom resource with EventBridge scheduling provides both deployment-time creation and ongoing reconciliation, matching the requirement for automatic lifecycle management.

## Alternatives Considered

- **Pipeline-managed static dashboard definition**
  - Rejected because it only updates when the pipeline runs and does not
    naturally keep dashboards current as tagged resources change.
- **CDK-defined static widgets**
  - Rejected because the resource inventory is dynamic. Hard-coded widgets would
    drift quickly and be harder to maintain.
- **Manual CloudWatch dashboard management**
  - Rejected because it is not Git-backed and does not meet the automation
    requirement.
- **One-time post-deploy script**
  - Rejected because it is weaker than the custom-resource plus schedule model
    for both deployment guarantees and ongoing refresh.

## Consequences

### Positive

- The dashboard is created during deployment and removed during destroy.
- The dashboard stays aligned with tagged resources over time.
- Dashboard behavior is managed in Git-backed code instead of console state.

### Trade-offs

- The design adds a Lambda, a custom resource path, and a schedule.
- Dashboard state is eventually consistent between scheduled refreshes.
- The implementation depends on stable tagging conventions across the platform.

## Impacted Domain(s)

- observe-plane/metrics

## Supporting Evidence/Evaluation

- [Metrics domain architecture document](../metrics-dasbboard.md)
- [Dashboard README](../../../metrics/dashboard/README.md)
- [Dashboard CDK construct](../../../../infra/metrics/dashboard/constructs/lambda.ts)
- [Dashboard Lambda handler](../../../metrics/dashboard/lambda/handler/handler.go)
- [Dashboard generator](../../../metrics/dashboard/dashboard.go)
