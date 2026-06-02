# service quota monitoring

monitors aws service quota usage and sends email alerts when usage exceeds a configured threshold (default 80%).

## architecture

```text
cloudwatch usage metrics (continuous, ~1 min resolution)
  -> cloudwatch alarm: (usage / SERVICE_QUOTA(usage)) * 100
    -> ALARM state: sns -> email notification (threshold exceeded)
```

this is the same mechanism as the **"Create alarm" button** in the service quotas console. no trusted advisor, step functions, or scheduled refreshes needed.

## dual-region deployment

aws only publishes usage metrics in the region where the resources exist. global services like IAM only publish to us-east-1. the stack deploys **two cloudformation stacks**, one per region:

| stack | region | alarms | services |
|---|---|---|---|
| `cmd-saas-service-quotas` | us-east-1 | 25 | IAM, CloudFormation, EC2, ELB, ASG, Lambda, S3, KMS, SSM, SNS, SES, RDS, DynamoDB |
| `cmd-saas-service-quotas` | us-east-2 | 8 | CloudFormation, AppSync, KMS, SSM, SES, DynamoDB |

each quota has a `deployRegions` field verified via AWS CLI `list-metrics` against both regions. shared services (CloudFormation, KMS, SSM, SES, DynamoDB) are monitored in both regions.

## how it works

aws publishes usage metrics continuously (~1 minute resolution). most quotas use the `AWS/Usage` cloudwatch namespace, while some (e.g. Lambda) publish to their own namespace. the cloudwatch alarm uses the built-in `SERVICE_QUOTA()` metric math function:

```text
(usage / SERVICE_QUOTA(usage)) * 100
```

the alarm fires **once** when usage crosses the threshold. it auto-resolves when usage drops back below but does not send a recovery email (CloudWatch OK actions can't distinguish ALARM→OK from INSUFFICIENT_DATA→OK, which would spam on every deployment).

## monitored quotas (26 unique, 33 alarms across regions)

| service | quotas monitored | region |
|---|---|---|
| **IAM** | roles, customer managed policies, instance profiles, users, OIDC providers | us-east-1 |
| **CloudFormation** | stacks | both |
| **EC2** | on-demand standard vCPUs | us-east-1 |
| **ELB** | ALBs, NLBs, CLBs, target groups | us-east-1 |
| **Auto Scaling** | auto scaling groups | us-east-1 |
| **Lambda** | concurrent executions | us-east-1 |
| **S3** | general purpose buckets | us-east-1 |
| **AppSync** | custom domain names | us-east-2 |
| **KMS** | customer master keys (CMKs) | both |
| **SSM** | standard parameters | both |
| **SNS** | topics per account | us-east-1 |
| **SES** | sending quota | both |
| **RDS** | DB instances, DB clusters, total storage, proxies | us-east-1 |
| **DynamoDB** | tables, provisioned read capacity, provisioned write capacity | both |

### removed quotas (zero usage in both regions)

these had no metric data in either region and were removed to keep the stack clean. re-add them if usage starts:

- IAM: groups, server certificates, SAML providers
- CloudFormation: stack sets
- EC2: elastic IPs
- VPC: interface endpoints
- Route 53: domain count (no domains registered via Route 53)
- AppSync: GraphQL APIs (Events API doesn't count as GraphQL)

### coverage gaps (no native usage metrics)

these services are used by cmd-saas but have no `UsageMetric` in the service quotas api:

| service | missing quota | risk | notes |
|---|---|---|---|
| **Amplify** | apps per region (25), domains per app (5) | HIGH | blocks at 5 tenants in non-prod layout |
| **Cognito** | user pools per region (1,000) | MEDIUM | 1 pool per tenant per env |
| **GuardDuty** | malware protection plans (100) | MEDIUM | 1 plan per tenant bucket |
| **API Gateway** | custom domain names (120), REST/HTTP APIs | HIGH | 2 domains per tenant |

## adding a new quota

1. check if the quota has a usage metric:

   ```bash
   aws service-quotas get-service-quota \
     --service-code <service-code> --quota-code <quota-code> \
     --query 'Quota.{Name:QuotaName,Usage:UsageMetric}'
   ```

2. verify the metric exists in your target region:

   ```bash
   aws cloudwatch list-metrics --namespace AWS/Usage \
     --dimensions Name=Service,Value=<service> Name=Resource,Value=<resource> \
     --region us-east-2
   ```

3. add an entry to `MONITORED_QUOTAS` in `service-quotas-stack.ts` with the correct `deployRegions`.

4. deploy.

## deployment

```bash
# install, build, deploy both stacks
corepack pnpm install --frozen-lockfile
corepack pnpm run build
corepack pnpm run deploy   # deploys to both us-east-1 and us-east-2

# or via Taskfile from repo root
task deploy-service-quotas
task deploy-service-quotas environment=prod
```

## useful commands

```bash
# check alarm states across both regions
for region in us-east-1 us-east-2; do
  echo "=== $region ==="
  aws cloudwatch describe-alarms \
    --alarm-name-prefix cmd-saas \
    --query 'MetricAlarms[].{name:AlarmName,state:StateValue}' \
    --output table --region $region
done

# verify metric availability before adding new quotas
aws cloudwatch list-metrics --namespace AWS/Usage \
  --dimensions Name=Service,Value=<service> Name=Resource,Value=<resource> \
  --region us-east-1

# check which quotas support usage metrics
aws service-quotas list-service-quotas --service-code <code> \
  --query 'Quotas[?UsageMetric].{Name:QuotaName,Code:QuotaCode}' \
  --output table
```
