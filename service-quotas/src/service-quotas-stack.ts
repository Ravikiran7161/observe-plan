import * as cdk from 'aws-cdk-lib';
import * as sns from 'aws-cdk-lib/aws-sns';
import * as subscriptions from 'aws-cdk-lib/aws-sns-subscriptions';
import * as cloudwatch from 'aws-cdk-lib/aws-cloudwatch';
import * as cw_actions from 'aws-cdk-lib/aws-cloudwatch-actions';
import { Construct } from 'constructs';
import { ResourceName, Plane, AwsService } from '@cmd-saas/libs-infra-resource';

/**
 * service quota definitions using cloudwatch metrics with SERVICE_QUOTA() math.
 *
 * most quotas use the AWS/Usage namespace — the same alarms you can create
 * manually via the service quotas console (Service Quotas -> AWS Services ->
 * <service> -> <quota> -> CloudWatch alarms).
 *
 * some quotas (e.g. Lambda concurrent executions) publish usage metrics in
 * their own namespace (AWS/Lambda) rather than AWS/Usage. these still work
 * with SERVICE_QUOTA() because they're linked through the service quotas api.
 * use the `namespace` and `dimensionsMap` overrides for these cases.
 *
 * IMPORTANT: aws only publishes usage metrics in regions where the resources
 * exist. global services (IAM) only publish to us-east-1. each quota has a
 * `deployRegions` field that controls which regional stack(s) include it.
 *
 * dimension values verified via:
 *   aws service-quotas list-service-quotas --service-code <code> \
 *     --query 'Quotas[?UsageMetric].{Name:QuotaName,Code:QuotaCode,Dims:UsageMetric}'
 *
 * COVERAGE GAPS (no native usage metrics — would need custom lambda):
 *   - Amplify: apps per region, domains per app
 *   - Cognito: user pools per region
 *   - GuardDuty: malware protection plans
 *   - API Gateway: custom domain names, REST/HTTP APIs
 *   - VPC: VPCs per region, Internet gateways, Subnets
 *   - EBS: gp3/gp2 storage (TiB), Active snapshots
 *   - Route53: Hosted zones
 *   - SQS: queues per region
 *   - ECS: clusters, services, tasks
 *   - OpenSearch: domains per region
 *   - ACM: certificates per region
 *   - Secrets Manager: secrets per region
 *
 * REMOVED (zero usage in both regions — re-add if usage starts):
 *   - IAM: Groups (L-F4A5425F), Server certificates (L-BF35879D), SAML providers (L-DB618D39)
 *   - CloudFormation: Stack sets (L-EC62D81A)
 *   - EC2: Elastic IPs (L-0263D0A3)
 *   - VPC: Interface endpoints (L-29B6F2EB)
 *   - Route 53: Domain count (L-F767CB15) — no domains registered via Route 53
 *   - AppSync: GraphQL APIs (L-4DFA3D2F) — Events API doesn't count as GraphQL
 */
interface QuotaDefinition {
  /** unique id for cdk construct (e.g. 'IAMRoles') */
  id: string;
  /** AWS/Usage metric dimensions — must match service quotas api output exactly */
  service: string;
  resource: string;
  type: 'Resource' | 'API';
  class: string;
  /** 'ResourceCount' for resource quotas, 'CallCount' for API/rate quotas */
  metricName: string;
  /** recommended statistic from service quotas api */
  statistic: string;
  /** alarm threshold as percentage of quota (e.g. 80 = 80%) */
  threshold: number;
  /** human-readable description for alarm — includes quota code for reference */
  description: string;
  /**
   * which region(s) to deploy this alarm in.
   * aws only publishes usage metrics where resources exist.
   * global services like IAM only publish to us-east-1.
   * verified via aws cloudwatch list-metrics.
   */
  deployRegions: ('us-east-1' | 'us-east-2')[];
  /**
   * override for quotas that publish usage metrics outside AWS/Usage namespace.
   * e.g. Lambda concurrent executions publishes to AWS/Lambda.
   * when set, `service`, `resource`, `type`, and `class` are ignored.
   */
  namespace?: string;
  /** custom dimensions map — use with namespace override for non-AWS/Usage metrics */
  dimensionsMap?: Record<string, string>;
}

const MONITORED_QUOTAS: QuotaDefinition[] = [
  // ============================================================
  // IAM — global service, metrics only publish to us-east-1
  // each tenant creates roles, policies, and instance profiles
  // ============================================================
  {
    id: 'IAMRoles',
    service: 'IAM', resource: 'Role', type: 'Resource', class: 'None',
    metricName: 'ResourceCount', statistic: 'Maximum',
    threshold: 80,
    description: 'IAM Roles per account (L-FE177D64)',
    deployRegions: ['us-east-1'],
  },
  {
    id: 'IAMPolicies',
    service: 'IAM', resource: 'CustomerManagedPolicy', type: 'Resource', class: 'None',
    metricName: 'ResourceCount', statistic: 'Maximum',
    threshold: 80,
    description: 'IAM Customer managed policies per account (L-E95E4862)',
    deployRegions: ['us-east-1'],
  },
  {
    id: 'IAMInstanceProfiles',
    service: 'IAM', resource: 'InstanceProfile', type: 'Resource', class: 'None',
    metricName: 'ResourceCount', statistic: 'Maximum',
    threshold: 80,
    description: 'IAM Instance profiles per account (L-6E65F664)',
    deployRegions: ['us-east-1'],
  },
  {
    id: 'IAMUsers',
    service: 'IAM', resource: 'User', type: 'Resource', class: 'None',
    metricName: 'ResourceCount', statistic: 'Maximum',
    threshold: 80,
    description: 'IAM Users per account (L-F55AF5E4)',
    deployRegions: ['us-east-1'],
  },
  {
    id: 'IAMOIDCProviders',
    service: 'IAM', resource: 'OIDCProvider', type: 'Resource', class: 'None',
    metricName: 'ResourceCount', statistic: 'Maximum',
    threshold: 80,
    description: 'IAM OpenID Connect providers per account (L-858F3967)',
    deployRegions: ['us-east-1'],
  },

  // ============================================================
  // CloudFormation — stacks exist in both regions
  // each tenant has ~6 stacks
  // ============================================================
  {
    id: 'CloudFormationStacks',
    service: 'CloudFormation', resource: 'Stack', type: 'Resource', class: 'None',
    metricName: 'ResourceCount', statistic: 'Maximum',
    threshold: 80,
    description: 'CloudFormation Stacks per account (L-0485CB21)',
    deployRegions: ['us-east-1', 'us-east-2'],
  },

  // ============================================================
  // EC2 — compute in us-east-1
  // ============================================================
  {
    id: 'EC2OnDemandStandard',
    service: 'EC2', resource: 'vCPU', type: 'Resource', class: 'Standard/OnDemand',
    metricName: 'ResourceCount', statistic: 'Maximum',
    threshold: 80,
    description: 'EC2 Running On-Demand Standard (A,C,D,H,I,M,R,T,Z) instances (L-1216C47A)',
    deployRegions: ['us-east-1'],
  },

  // ============================================================
  // Elastic Load Balancing — infra in us-east-1
  // ============================================================
  {
    id: 'ALBsPerRegion',
    service: 'Elastic Load Balancing', resource: 'ApplicationLoadBalancersPerRegion', type: 'Resource', class: 'None',
    metricName: 'ResourceCount', statistic: 'Maximum',
    threshold: 80,
    description: 'Application Load Balancers per Region (L-53DA6B97)',
    deployRegions: ['us-east-1'],
  },
  {
    id: 'NLBsPerRegion',
    service: 'Elastic Load Balancing', resource: 'NetworkLoadBalancersPerRegion', type: 'Resource', class: 'None',
    metricName: 'ResourceCount', statistic: 'Maximum',
    threshold: 80,
    description: 'Network Load Balancers per Region (L-69A177A2)',
    deployRegions: ['us-east-1'],
  },
  {
    id: 'CLBsPerRegion',
    service: 'Elastic Load Balancing', resource: 'ClassicLoadBalancersPerRegion', type: 'Resource', class: 'None',
    metricName: 'ResourceCount', statistic: 'Maximum',
    threshold: 80,
    description: 'Classic Load Balancers per Region (L-E9E9831D)',
    deployRegions: ['us-east-1'],
  },
  {
    id: 'TargetGroupsPerRegion',
    service: 'Elastic Load Balancing', resource: 'TargetGroupsPerRegion', type: 'Resource', class: 'None',
    metricName: 'ResourceCount', statistic: 'Maximum',
    threshold: 80,
    description: 'Target Groups per Region (L-B22855CB)',
    deployRegions: ['us-east-1'],
  },

  // ============================================================
  // Auto Scaling — us-east-1
  // ============================================================
  {
    id: 'AutoScalingGroups',
    service: 'AutoScaling', resource: 'NumberOfAutoScalingGroup', type: 'Resource', class: 'None',
    metricName: 'ResourceCount', statistic: 'Maximum',
    threshold: 80,
    description: 'Auto Scaling groups per region (L-CDE20ADC)',
    deployRegions: ['us-east-1'],
  },

  // ============================================================
  // Lambda — concurrent executions across all functions
  // uses AWS/Lambda namespace (not AWS/Usage) with no dimensions.
  // default limit 1,000 — critical for multi-tenant burst traffic.
  // ============================================================
  {
    id: 'LambdaConcurrentExecutions',
    service: 'Lambda', resource: '', type: 'Resource', class: 'None',
    namespace: 'AWS/Lambda',
    metricName: 'ConcurrentExecutions',
    dimensionsMap: {},
    statistic: 'Maximum',
    threshold: 80,
    description: 'Lambda Concurrent executions (L-B99A9384)',
    deployRegions: ['us-east-1'],
  },

  // ============================================================
  // S3 — one bucket per tenant per environment
  // global service, metrics in us-east-1
  // default limit 100 — hits at ~33 tenants across 3 environments
  // ============================================================
  {
    id: 'S3GeneralPurposeBuckets',
    service: 'S3', resource: 'GeneralPurposeBuckets', type: 'Resource', class: 'None',
    metricName: 'ResourceCount', statistic: 'Maximum',
    threshold: 80,
    description: 'S3 General purpose buckets (L-DC2B2D3D)',
    deployRegions: ['us-east-1'],
  },

  // ============================================================
  // AppSync — notification events api
  // domain names verified in us-east-2 only
  // ============================================================
  {
    id: 'AppSyncDomainNames',
    service: 'AppSync', resource: 'DomainNames', type: 'Resource', class: 'None',
    metricName: 'ResourceCount', statistic: 'Maximum',
    threshold: 80,
    description: 'AppSync Custom domain names (L-51A37BC6)',
    deployRegions: ['us-east-2'],
  },

  // ============================================================
  // KMS — encryption keys in both regions
  // ============================================================
  {
    id: 'KMSKeys',
    service: 'KMS', resource: 'KeysPerAccount', type: 'Resource', class: 'None',
    metricName: 'ResourceCount', statistic: 'Maximum',
    threshold: 80,
    description: 'KMS Customer Master Keys per account (L-C2F1777E)',
    deployRegions: ['us-east-1', 'us-east-2'],
  },

  // ============================================================
  // SSM Parameter Store — parameters in both regions
  // 5-10 parameters per tenant
  // ============================================================
  {
    id: 'SSMStandardParameters',
    service: 'SSM', resource: 'StandardParameterCount', type: 'Resource', class: 'None',
    metricName: 'ResourceCount', statistic: 'Maximum',
    threshold: 80,
    description: 'SSM Standard parameters (L-C3B871CB)',
    deployRegions: ['us-east-1', 'us-east-2'],
  },

  // ============================================================
  // SNS — topics in us-east-1
  // ============================================================
  {
    id: 'SNSTopics',
    service: 'SNS', resource: 'ApproximateNumberOfTopics', type: 'Resource', class: 'None',
    metricName: 'ResourceCount', statistic: 'Maximum',
    threshold: 80,
    description: 'SNS Topics per account (L-61103206)',
    deployRegions: ['us-east-1'],
  },

  // ============================================================
  // SES — email sending in both regions
  // ============================================================
  {
    id: 'SESSendingQuota',
    service: 'SES', resource: 'SendLast24Hours', type: 'API', class: 'None',
    metricName: 'CallCount', statistic: 'Maximum',
    threshold: 80,
    description: 'SES Sending quota (L-804C8AE8)',
    deployRegions: ['us-east-1', 'us-east-2'],
  },

  // ============================================================
  // RDS — databases in us-east-1
  // ============================================================
  {
    id: 'RDSInstances',
    service: 'RDS', resource: 'DBInstances', type: 'Resource', class: 'None',
    metricName: 'ResourceCount', statistic: 'Maximum',
    threshold: 80,
    description: 'RDS DB instances (L-7B6409FD)',
    deployRegions: ['us-east-1'],
  },
  {
    id: 'RDSClusters',
    service: 'RDS', resource: 'DBClusters', type: 'Resource', class: 'None',
    metricName: 'ResourceCount', statistic: 'Maximum',
    threshold: 80,
    description: 'RDS DB clusters (L-952B80B8)',
    deployRegions: ['us-east-1'],
  },
  {
    id: 'RDSStorage',
    service: 'RDS', resource: 'AllocatedStorage', type: 'Resource', class: 'None',
    metricName: 'ResourceCount', statistic: 'Maximum',
    threshold: 80,
    description: 'RDS Total storage for all DB instances (L-7ADDB58A)',
    deployRegions: ['us-east-1'],
  },
  {
    id: 'RDSProxies',
    service: 'RDS', resource: 'Proxies', type: 'Resource', class: 'None',
    metricName: 'ResourceCount', statistic: 'Maximum',
    threshold: 80,
    description: 'RDS Proxies (L-D94C7EA3)',
    deployRegions: ['us-east-1'],
  },

  // ============================================================
  // DynamoDB — tables and throughput in both regions
  // ============================================================
  {
    id: 'DynamoDBTables',
    service: 'DynamoDB', resource: 'TableCount', type: 'Resource', class: 'None',
    metricName: 'ResourceCount', statistic: 'Maximum',
    threshold: 80,
    description: 'DynamoDB Maximum number of tables (L-F98FE922)',
    deployRegions: ['us-east-1', 'us-east-2'],
  },
  {
    id: 'DynamoDBReadCapacity',
    service: 'DynamoDB', resource: 'AccountProvisionedReadCapacityUnits', type: 'Resource', class: 'None',
    metricName: 'ResourceCount', statistic: 'Maximum',
    threshold: 80,
    description: 'DynamoDB Account-level read throughput limit (L-34F6A552)',
    deployRegions: ['us-east-1', 'us-east-2'],
  },
  {
    id: 'DynamoDBWriteCapacity',
    service: 'DynamoDB', resource: 'AccountProvisionedWriteCapacityUnits', type: 'Resource', class: 'None',
    metricName: 'ResourceCount', statistic: 'Maximum',
    threshold: 80,
    description: 'DynamoDB Account-level write throughput limit (L-34F8CCC8)',
    deployRegions: ['us-east-1', 'us-east-2'],
  },
];

/** get quotas that should be deployed in a given region */
export function getQuotasForRegion(region: string): QuotaDefinition[] {
  return MONITORED_QUOTAS.filter((q) => q.deployRegions.includes(region as 'us-east-1' | 'us-east-2'));
}

/** get all monitored quotas (for documentation/outputs) */
export function getAllQuotas(): QuotaDefinition[] {
  return MONITORED_QUOTAS;
}

export interface ServiceQuotasStackProps extends cdk.StackProps {
  environmentName: string;
  notificationEmails: string[];
  /** deploy region — only quotas matching this region are created */
  deployRegion: string;
}

/**
 * service quota monitoring using native cloudwatch metrics with SERVICE_QUOTA() math.
 *
 * architecture:
 *   cloudwatch alarm (usage metric + SERVICE_QUOTA() math)
 *     -> sns -> email (on ALARM and OK state transitions)
 *
 * most quotas use the AWS/Usage namespace — the same mechanism as the
 * "Create alarm" button in the service quotas console. some quotas
 * (e.g. Lambda) publish to their own namespace but still work with
 * SERVICE_QUOTA() because they're linked through the service quotas api.
 *
 * IMPORTANT: deploy one stack per region. aws only publishes usage metrics
 * in the region where the resources exist. global services like IAM only
 * publish to us-east-1. each quota has a deployRegions field verified
 * via aws cloudwatch list-metrics.
 *
 * the alarm fires ONCE when usage crosses the threshold. it auto-resolves
 * when usage drops back below but does not send a recovery email (CloudWatch
 * OK actions can't distinguish ALARM->OK from INSUFFICIENT_DATA->OK).
 */
export class ServiceQuotasStack extends cdk.Stack {
  public readonly alertTopic: sns.Topic;

  constructor(scope: Construct, id: string, props: ServiceQuotasStackProps) {
    super(scope, id, props);

    const env = props.environmentName;
    const region = props.deployRegion;
    const quotas = getQuotasForRegion(region);

    // --- sns topic ---

    const topicName = new ResourceName({
      plane: Plane.OP,
      environment: env,
      domain: 'svc-quotas',
      purposeDescriptor: 'alerts',
      awsService: AwsService.SNS,
      region,
    }).toString();

    this.alertTopic = new sns.Topic(this, 'QuotaAlertsTopic', {
      topicName,
      displayName: `Service Quota Alerts - ${env} (${region})`,
    });

    for (const email of props.notificationEmails) {
      this.alertTopic.addSubscription(
        new subscriptions.EmailSubscription(email),
      );
    }

    // --- cloudwatch alarms ---

    for (const quota of quotas) {
      this.createQuotaAlarm(env, region, quota);
    }

    // --- tags ---

    cdk.Tags.of(this).add('Project', 'cmd-saas');
    cdk.Tags.of(this).add('Domain', 'svc-quotas');
    cdk.Tags.of(this).add('Environment', env);
    cdk.Tags.of(this).add('Region', region);
    cdk.Tags.of(this).add('ManagedBy', 'cdk');

    // --- outputs ---

    new cdk.CfnOutput(this, 'AlertTopicArn', {
      value: this.alertTopic.topicArn,
      description: 'sns topic for quota alerts',
    });

    new cdk.CfnOutput(this, 'MonitoredQuotas', {
      value: `${quotas.length} quotas in ${region}: ${[...new Set(quotas.map((q) => q.service))].join(', ')}`,
      description: 'number and services of monitored quotas in this region',
    });
  }

  /**
   * create a cloudwatch alarm using the SERVICE_QUOTA() metric math function.
   *
   * the expression (usage / SERVICE_QUOTA(usage)) * 100 calculates usage as a
   * percentage of the quota limit. SERVICE_QUOTA() is a built-in cloudwatch
   * function that returns the current quota value for a given usage metric.
   *
   * quotas with a `namespace` override use their own namespace + dimensions
   * (e.g. Lambda ConcurrentExecutions → AWS/Lambda with no dimensions).
   * all others use standard AWS/Usage namespace with Service/Resource/Type/Class.
   *
   * see: https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/CloudWatch-Quotas-Visualize-Alarms.html
   */
  private createQuotaAlarm(env: string, region: string, quota: QuotaDefinition): cloudwatch.Alarm {
    const slug = quota.id.replace(/([a-z])([A-Z])/g, '$1-$2').toLowerCase();

    const alarmName = new ResourceName({
      plane: Plane.OP,
      environment: env,
      domain: 'svc-quotas',
      purposeDescriptor: slug,
      awsService: AwsService.CLOUDFORMATION,
      region,
    }).toString();

    const usageMetric = quota.namespace
      ? new cloudwatch.Metric({
          namespace: quota.namespace,
          metricName: quota.metricName,
          dimensionsMap: quota.dimensionsMap ?? {},
          statistic: quota.statistic,
        })
      : new cloudwatch.Metric({
          namespace: 'AWS/Usage',
          metricName: quota.metricName,
          dimensionsMap: {
            Service: quota.service,
            Resource: quota.resource,
            Type: quota.type,
            Class: quota.class,
          },
          statistic: quota.statistic,
        });

    const percentExpression = new cloudwatch.MathExpression({
      expression: '(usage / SERVICE_QUOTA(usage)) * 100',
      label: `${quota.description} (% of quota)`,
      usingMetrics: { usage: usageMetric },
      period: cdk.Duration.minutes(5),
    });

    const alarm = percentExpression.createAlarm(this, quota.id, {
      alarmName,
      alarmDescription: `${quota.description} — usage exceeds ${quota.threshold}% of quota`,
      threshold: quota.threshold,
      evaluationPeriods: 1,
      comparisonOperator: cloudwatch.ComparisonOperator.GREATER_THAN_OR_EQUAL_TO_THRESHOLD,
      treatMissingData: cloudwatch.TreatMissingData.NOT_BREACHING,
    });

    alarm.addAlarmAction(new cw_actions.SnsAction(this.alertTopic));

    return alarm;
  }
}
