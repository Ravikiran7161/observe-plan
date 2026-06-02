#!/usr/bin/env node
import 'source-map-support/register';
import * as cdk from 'aws-cdk-lib';
import { ResourceName, Plane, AwsService, Region } from '@cmd-saas/libs-infra-resource';
import { ServiceQuotasStack, getAllQuotas, getQuotasForRegion } from './service-quotas-stack';

/**
 * service quotas monitoring — dual-region deployment.
 *
 * creates two cloudformation stacks, one per region:
 *   - op-use1-<env>-svc-quotas: IAM, EC2, ELB, ASG, Lambda, S3, SNS, RDS + shared
 *   - op-use2-<env>-svc-quotas: AppSync + shared (CloudFormation, KMS, SSM, SES, DynamoDB)
 *
 * naming convention: op-<regionAbbr>-<env>-svc-quotas
 *   plane=op (observe plane), domain=svc-quotas
 *
 * each stack only contains alarms for quotas whose metrics are published
 * in that region. verified via aws cloudwatch list-metrics.
 *
 * environment variables:
 *   ENVIRONMENT         - environment name (required)
 *   NOTIFICATION_EMAILS - comma-separated list of emails
 *   AWS_ACCOUNT         - target aws account id
 *
 * environment can also be passed via CDK context: -c environment=main
 */

const app = new cdk.App();

const environmentName: string =
  app.node.tryGetContext('environment') ?? process.env.ENVIRONMENT ?? '';
if (!environmentName) {
  throw new Error('Environment is required. Pass via CDK context (-c environment=main) or ENVIRONMENT env var.');
}

const notificationEmailsRaw =
  process.env.NOTIFICATION_EMAILS || 'cmd-aws-alerts@thermofisher.onmicrosoft.com';
const notificationEmails = notificationEmailsRaw.split(',').map((e) => e.trim());

const account = process.env.AWS_ACCOUNT || process.env.CDK_DEFAULT_ACCOUNT;

const regions = [Region.US_EAST_1, Region.US_EAST_2];

for (const region of regions) {
  const quotas = getQuotasForRegion(region);
  if (quotas.length === 0) continue;

  const stackName = new ResourceName({
    plane: Plane.OP,
    environment: environmentName,
    domain: 'svc-quotas',
    awsService: AwsService.CLOUDFORMATION,
    region,
  }).toString();

  new ServiceQuotasStack(app, stackName, {
    stackName,
    environmentName,
    notificationEmails,
    deployRegion: region,
    env: { account, region },
  });
}

// log summary
const allQuotas = getAllQuotas();
const total = new Set<string>();
for (const region of regions) {
  const quotas = getQuotasForRegion(region);
  quotas.forEach((q) => total.add(q.id));
  console.log(`${region}: ${quotas.length} alarms (${[...new Set(quotas.map((q) => q.service))].join(', ')})`);
}
console.log(`total unique quotas: ${total.size}`);

app.synth();
