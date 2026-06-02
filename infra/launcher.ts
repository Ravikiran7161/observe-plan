import * as cdk from 'aws-cdk-lib';
import { ResourceName, AwsService, Plane } from '@cmd-saas/libs-infra-resource';
import { ObservePlaneDomain } from './domains.js';
import { DashboardStack } from './metrics/dashboard/stacks/dashboard.js';

const app = new cdk.App();

const environmentContext = app.node.tryGetContext('environment');
const environment = environmentContext || process.env.NODE_ENV;
if (!environment) {
  throw new Error('Environment is required. Provide with -c environment=<env> or NODE_ENV');
}

const tenantCodeContext = app.node.tryGetContext('tenant_code') || app.node.tryGetContext('tenant_id');
const tenantCode = typeof tenantCodeContext === 'string' ? tenantCodeContext.trim() : '';

const dashboardTags: Record<string, string> = {
  environment: environment,
};

if (tenantCode) {
  dashboardTags['tenant-code'] = tenantCode;
}

const dashboardPurposeDescriptor = tenantCode ? `dashboard-${tenantCode}` : 'dashboard';

const stackName = new ResourceName({
  plane: Plane.OP,
  environment,
  domain: ObservePlaneDomain.METRICS,
  purposeDescriptor: dashboardPurposeDescriptor,
  awsService: AwsService.CLOUDFORMATION,
}).toString();

const dashboardStack = new DashboardStack(app, stackName, {
  stackName,
  env: {
    account: process.env.CDK_DEFAULT_ACCOUNT,
    region: process.env.CDK_DEFAULT_REGION || 'us-east-1',
  },
  environment,
  tenantID: tenantCode || undefined,
  dashboardTags,
  description: 'Observe Plane - dynamic CloudWatch dashboard showing metrics',
});

// Suppress unused variable warning for metricsStack
void dashboardStack;

app.synth();
