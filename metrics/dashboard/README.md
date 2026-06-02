# Dashboard

This directory contains the dashboard manager used by the Observe Plane metrics domain. It dynamically creates and updates a custom CloudWatch dashboard with widgets displaying various metrics about SaaS resources. Widgets are scoped by resource tags. By default, the dashboard is scoped to `environment=<saas_env>` tag. Optionally, add `tenant_code=<tenant_code>` to create a tenant-specific dashboard within the same environment.

For the architecture overview, see [`observe-plane/docs/architecture/metrics-dasbboard.md`](../../docs/architecture/metrics-dasbboard.md). For the design decision, see [`observe-plane/docs/architecture/adr/001-adr-dynamic-dashboard-management.md`](../../docs/architecture/adr/001-adr-dynamic-dashboard-management.md).

## Deploy

From the repository root:

```sh
task validate-metrics-dashboard saas_env=main
```

Then deploy:

```sh
task deploy-metrics-dashboard saas_env=main
```

To deploy a tenant-specific dashboard for the same environment:

```sh
task deploy-metrics-dashboard saas_env=main tenant_code=devcorp
```

You can also run the GitHub Actions workflow
[`Metrics Dashboard Management`](../../../.github/workflows/metrics-dashboard-management.yaml).
Use `operation=deploy`, choose the matching GitHub environment for AWS
credentials, and optionally provide `tenant_code` for a tenant-scoped
dashboard.

Pull requests that change dashboard-related files now run
[`Metrics Dashboard Validation`](../../../.github/workflows/metrics-dashboard-validation.yaml)
automatically, and pushes to `main` auto-deploy the shared dashboard for
`saas_env=main`.

## Destroy

From the repository root:

```sh
task destroy-metrics-dashboard saas_env=main
```

To destroy a tenant-specific dashboard:

```sh
task destroy-metrics-dashboard saas_env=main tenant_code=devcorp
```

The same GitHub Actions workflow supports `operation=destroy` and passes
`force=true` to the Task target after rebuilding the observe-plane CDK package
for a clean runner.
