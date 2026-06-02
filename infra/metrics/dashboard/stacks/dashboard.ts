import * as cdk from 'aws-cdk-lib';
import * as events from 'aws-cdk-lib/aws-events';
import { Construct } from 'constructs';
import { DashboardLambdaResources } from '../constructs/lambda.js';

export interface DashboardStackProps extends cdk.StackProps {
  readonly environment: string;
  readonly tenantID?: string;
  readonly dashboardTags: Record<string, string>;
  readonly schedule?: events.Schedule;
}

export class DashboardStack extends cdk.Stack {
  constructor(scope: Construct, id: string, props: DashboardStackProps) {
    super(scope, id, props);

    new DashboardLambdaResources(this, 'DashboardLambdaResources', {
      environment: props.environment,
      tenantID: props.tenantID,
      dashboardTags: props.dashboardTags,
      schedule: props.schedule,
    });
  }
}
