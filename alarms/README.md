# Alarms

CloudWatch alarm management for the Observe Plane. Discovers tagged AWS resources, groups them by scope (control plane, app plane domains, tenants), and deploys one CloudFormation alarm stack per scope.

## How It Works

```
config/main.json          → tags to query (e.g. environment=main)
        │
        ▼
discover (Go CLI)         → queries AWS, groups resources by plane/domain/tenant-code
        │
        ▼
generated/multi/          → one inventory file per scope + manifest.json
        │
        ▼
launcher.ts (CDK)         → reads manifest, creates one AlarmsStack per scope
        │
        ▼
CloudFormation            → one stack per scope with CloudWatch alarms + SNS topic
```

## Commands

```sh
# Deploy all alarm stacks for an environment
task deploy-alarms config=observe-plane/alarms/config/main.json

# Destroy all alarm stacks for an environment
task destroy-alarms config=observe-plane/alarms/config/main.json force=true

# Destroy a specific tenant's alarm stack
task destroy-tenant-alarms config=observe-plane/alarms/config/main.json tenant=devcorp
```

## Scope Grouping

Resources are grouped into stacks based on their AWS tags:

| Resource tags | Scope | Stack name example |
|---|---|---|
| `plane=control` | control | `op-use2-main-metrics-alarms-control` |
| `plane=app, domain=drive` | app-drive | `op-use2-main-metrics-alarms-app-drive` |
| `tenant-code=devcorp` | tenant-devcorp | `op-use2-main-metrics-alarms-tenant-devcorp` |

New tenants are auto-discovered on the next deploy — no config change needed.

## Alarms Created

### Lambda

- **Errors** — function throwing exceptions
- **Throttles** — concurrency limit hit
- **Duration p95** — slow execution
- **Concurrent Executions** — approaching account limit

### API Gateway

- **4xx errors** — client errors (bad requests, auth failures)
- **5xx errors** — server errors (backend broken)
- **Latency p95** — slow responses

### Amplify

- **4xx errors** — client errors on the hosted app
- **5xx errors** — server errors on the hosted app
- **Latency p95** — time to first byte (slow page loads)
- **Requests** — unusual traffic spike
- **Tokens Consumed** — high consumption causes throttling

### Cognito

- **Throttle Count** — API being hammered
- **Sign-in Throttles** — brute force or credential stuffing indicator

### DynamoDB

- **System Errors** — AWS-side failures (table unavailable)
- **Throttled Requests** — read/write capacity exceeded
- **Request Latency p99** — slow reads or writes

## Alarm Naming

```
op-{region}-{environment}-alarms-{service}-{resource-name}-{metric}
```

## Overrides

Optional JSON files in `overrides/` allow per-resource threshold customization without code changes. See individual override files for format and examples.

Threshold precedence: **per-resource override > defaults block in file > hardcoded fallback**

## SNS Notifications

One SNS topic per stack. All alarms notify the email address defined in `config/<env>.json`. After first deploy, confirm the SNS subscription email.

## Tenant Lifecycle

- **Tenant provisioned** → next `deploy-alarms` run automatically creates a new stack for it
- **Tenant deleted** → run `task destroy-tenant-alarms tenant=<code>` to remove its alarm stack
