import { Construct } from 'constructs';
import { Stack } from 'aws-cdk-lib';
import * as cdk from 'aws-cdk-lib';
import * as iam from 'aws-cdk-lib/aws-iam';
import * as lambda from 'aws-cdk-lib/aws-lambda';
import * as logs from 'aws-cdk-lib/aws-logs';
import * as events from 'aws-cdk-lib/aws-events';
import * as targets from 'aws-cdk-lib/aws-events-targets';
import * as cr from 'aws-cdk-lib/custom-resources';
import * as path from 'path';
import { GoLambdaFunction } from '@cmd-saas/libs-infra-constructs';
import { ResourceName, AwsService, Plane } from '@cmd-saas/libs-infra-resource';
import { ObservePlaneDomain } from '../../../domains.js';

// Resolve repository root from the location of this file:
// observe-plane/infra/metrics/dashboard/constructs/lambda.ts → ../../../../../
const repoRoot = path.resolve(new URL(import.meta.url).pathname, '../../../../../..');
export interface DashboardLambdaResourcesProps {
  readonly environment: string;
  readonly tenantID?: string;
  /** Tag filters used to discover resources for this dashboard (e.g. { environment: 'dev' }). */
  readonly dashboardTags: Record<string, string>;
  /** EventBridge schedule expression. Defaults to every hour. */
  readonly schedule?: events.Schedule;
}

interface DashboardCustomResourceProperties {
  readonly functionName: string;
  readonly physicalResourceId: string;
  readonly tags: Record<string, string>;
  readonly updateToken: string;
}

export class DashboardLambdaResources extends Construct {
  public readonly lambdaFunction: lambda.IFunction;

  constructor(scope: Construct, id: string, props: DashboardLambdaResourcesProps) {
    super(scope, id);

    const { environment, tenantID, dashboardTags, schedule } = props;
    const purposeDescriptor = tenantID ? `dashboard-${tenantID}` : 'dashboard';
    const descriptionSuffix = tenantID ? `${environment}, tenant=${tenantID}` : environment;

    this.lambdaFunction = new GoLambdaFunction(this, 'DashboardLambda', {
      entry: path.resolve(repoRoot, 'observe-plane/metrics/dashboard/lambda/handler'),
      moduleDir: path.resolve(repoRoot, 'observe-plane/metrics/dashboard/go.mod'),
      functionName: new ResourceName({
        plane: Plane.OP,
        environment,
        domain: ObservePlaneDomain.METRICS,
        purposeDescriptor,
        awsService: AwsService.LAMBDA,
        scope: Stack.of(this),
      }),
      timeout: cdk.Duration.minutes(5),
      memorySize: 256,
      description: `Dynamic CloudWatch dashboard creator - ${descriptionSuffix}`,
      loggingFormat: lambda.LoggingFormat.JSON,
      logGroupRetention: logs.RetentionDays.ONE_WEEK,
    });
    const dashboardLambda = this.lambdaFunction as lambda.Function;

    // Least-privilege IAM permissions for all resource discovery + dashboard management
    this.lambdaFunction.addToRolePolicy(new iam.PolicyStatement({
      actions: [
        'cloudwatch:PutDashboard',
        'cloudwatch:DeleteDashboards',
        'cloudwatch:ListDashboards',
      ],
      resources: ['*'],
    }));

    this.lambdaFunction.addToRolePolicy(new iam.PolicyStatement({
      actions: ['tag:GetResources'],
      resources: ['*'],
    }));

    this.lambdaFunction.addToRolePolicy(new iam.PolicyStatement({
      actions: ['amplify:ListApps'],
      resources: ['*'],
    }));

    this.lambdaFunction.addToRolePolicy(new iam.PolicyStatement({
      actions: [
        'cognito-idp:ListUserPools',
        'cognito-idp:ListUserPoolClients',
        'cognito-idp:ListTagsForResource',
      ],
      resources: ['*'],
    }));

    this.lambdaFunction.addToRolePolicy(new iam.PolicyStatement({
      actions: ['apigateway:GET'],
      resources: ['arn:aws:apigateway:*::/*'],
    }));

    const customResourceHandlerLogGroup = new logs.LogGroup(this, 'DashboardCustomResourceHandlerLogGroup', {
      retention: logs.RetentionDays.ONE_WEEK,
      removalPolicy: cdk.RemovalPolicy.DESTROY,
    });

    const customResourceHandler = new lambda.Function(this, 'DashboardCustomResourceHandler', {
      runtime: lambda.Runtime.PYTHON_3_13,
      handler: 'index.handler',
      timeout: cdk.Duration.minutes(5),
      code: lambda.Code.fromInline(`
import json
import boto3

lambda_client = boto3.client("lambda")

def handler(event, _context):
    print(json.dumps(event))
    request_type = event["RequestType"]
    props = event.get("ResourceProperties", {})
    payload = {
        "tags": props.get("tags", {}),
        "delete": request_type == "Delete",
    }

    response = lambda_client.invoke(
        FunctionName=props["functionName"],
        InvocationType="RequestResponse",
        Payload=json.dumps(payload).encode("utf-8"),
    )

    function_error = response.get("FunctionError")
    payload_bytes = response["Payload"].read()
    payload_text = payload_bytes.decode("utf-8") if payload_bytes else ""

    if function_error:
        raise RuntimeError(
            f"Dashboard Lambda failed with {function_error}: {payload_text}"
        )

    return {
        "PhysicalResourceId": props["physicalResourceId"],
        "Data": {
            "Payload": payload_text,
        },
    }
`),
      logGroup: customResourceHandlerLogGroup,
    });

    customResourceHandler.addToRolePolicy(new iam.PolicyStatement({
      actions: ['lambda:InvokeFunction'],
      resources: [dashboardLambda.functionArn],
    }));

    const customResourceProviderLogGroup = new logs.LogGroup(this, 'DashboardCustomResourceProviderLogGroup', {
      retention: logs.RetentionDays.ONE_WEEK,
      removalPolicy: cdk.RemovalPolicy.DESTROY,
    });

    const customResourceProvider = new cr.Provider(this, 'DashboardCustomResourceProvider', {
      onEventHandler: customResourceHandler,
      logGroup: customResourceProviderLogGroup,
    });

    // Invoke the Lambda immediately on create/update so the dashboard exists right away,
    // without waiting for the first EventBridge tick. Using a provider-style custom
    // resource ensures any Lambda handler error fails the CloudFormation operation.
    new cdk.CustomResource(this, 'InvokeOnDeploy', {
      serviceToken: customResourceProvider.serviceToken,
      properties: {
        functionName: dashboardLambda.functionName,
        physicalResourceId: `dashboard-${environment}${tenantID ? `-${tenantID}` : ''}`,
        tags: dashboardTags,
        updateToken: dashboardLambda.currentVersion.functionArn,
      } satisfies DashboardCustomResourceProperties,
    });

    new events.Rule(this, 'ScheduleRule', {
      schedule: schedule ?? events.Schedule.rate(cdk.Duration.hours(1)),
      description: `Refresh dashboard for tags: ${JSON.stringify(dashboardTags)}`,
      targets: [
        new targets.LambdaFunction(this.lambdaFunction, {
          event: events.RuleTargetInput.fromObject({
            tags: dashboardTags,
            delete: false,
          }),
        }),
      ],
    });
  }
}
