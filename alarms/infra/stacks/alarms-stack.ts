import * as cdk from 'aws-cdk-lib';
import * as path from 'path';
import { fileURLToPath } from 'url';
import { Construct } from 'constructs';
import { SnsNotification } from '../constructs/sns-notification.js';
import { CommonApiGatewayAlarms } from '../constructs/common-apigateway-alarms.js';
import { CommonAmplifyAlarms } from '../constructs/common-amplify-alarms.js';
import { CommonLambdaAlarms } from '../constructs/common-lambda-alarms.js';
import { CommonCognitoAlarms } from '../constructs/common-cognito-alarms.js';
import { CommonDynamoDBAlarms } from '../constructs/common-dynamodb-alarms.js';
import {
  ResourceInventory,
  ApiGatewayOverrides,
  AmplifyOverrides,
  LambdaOverrides,
  CognitoOverrides,
  DynamoDBOverrides,
  loadApiGatewayOverrides,
  loadAmplifyOverrides,
  loadLambdaOverrides,
  loadCognitoOverrides,
  loadDynamoDBOverrides,
} from '../alarm-config.js';

// Resolve overrides directory relative to this file:
// observe-plane/alarms/infra/stacks/alarms-stack.ts → ../../overrides/
const overridesDir = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../overrides');

export interface AlarmsStackProps extends cdk.StackProps {
  /** Environment name (e.g. 'main', 'dev'). */
  readonly environment: string;
  /** Email address for alarm notifications. */
  readonly alarmEmail: string;
  /** Resource inventory produced by the discover CLI. */
  readonly inventory: ResourceInventory;
}

/**
 * Single CloudFormation stack for all CloudWatch alarms in one environment.
 * Naming follows the same ResourceName convention as the dashboard stack.
 *
 * Contents:
 *   - SnsNotification          — shared SNS topic + email subscription
 *   - CommonApiGatewayAlarms   — 4xx, 5xx, latency p95 alarms per API+stage
 */
export class AlarmsStack extends cdk.Stack {
  public readonly snsNotification: SnsNotification;

  constructor(scope: Construct, id: string, props: AlarmsStackProps) {
    super(scope, id, props);

    const { environment, alarmEmail, inventory } = props;

    // Shared SNS topic — one per environment, used by all alarm constructs.
    this.snsNotification = new SnsNotification(this, 'SnsNotification', {
      environment,
      alarmEmail,
    });

    // Load optional API Gateway overrides file.
    const apiGatewayOverrides: ApiGatewayOverrides | undefined = loadApiGatewayOverrides(
      path.join(overridesDir, 'apigateway.json'),
    );

    // API Gateway alarms — one set of alarms per API+stage in the inventory.
    if (inventory.resources.apiGateways.length > 0) {
      new CommonApiGatewayAlarms(this, 'ApiGatewayAlarms', {
        apis: inventory.resources.apiGateways,
        alarmTopic: this.snsNotification.topic,
        overrides: apiGatewayOverrides,
        environment,
        region: this.region,
      });
    }

    // Load optional Amplify overrides file.
    const amplifyOverrides: AmplifyOverrides | undefined = loadAmplifyOverrides(
      path.join(overridesDir, 'amplify.json'),
    );

    // Amplify alarms — one set of alarms per app in the inventory.
    if (inventory.resources.amplifyApps.length > 0) {
      new CommonAmplifyAlarms(this, 'AmplifyAlarms', {
        apps: inventory.resources.amplifyApps,
        alarmTopic: this.snsNotification.topic,
        overrides: amplifyOverrides,
        environment,
        region: this.region,
      });
    }

    // Load optional Lambda overrides file.
    const lambdaOverrides: LambdaOverrides | undefined = loadLambdaOverrides(
      path.join(overridesDir, 'lambda.json'),
    );

    // Lambda alarms — one set of alarms per function in the inventory.
    if (inventory.resources.lambdas.length > 0) {
      new CommonLambdaAlarms(this, 'LambdaAlarms', {
        functions: inventory.resources.lambdas,
        alarmTopic: this.snsNotification.topic,
        overrides: lambdaOverrides,
        environment,
        region: this.region,
      });
    }

    // Load optional Cognito overrides file.
    const cognitoOverrides: CognitoOverrides | undefined = loadCognitoOverrides(
      path.join(overridesDir, 'cognito.json'),
    );

    // Cognito alarms — one set of alarms per user pool in the inventory.
    if (inventory.resources.cognitoUserPools.length > 0) {
      new CommonCognitoAlarms(this, 'CognitoAlarms', {
        pools: inventory.resources.cognitoUserPools,
        alarmTopic: this.snsNotification.topic,
        overrides: cognitoOverrides,
        environment,
        region: this.region,
      });
    }

    // Load optional DynamoDB overrides file.
    const dynamoDBOverrides: DynamoDBOverrides | undefined = loadDynamoDBOverrides(
      path.join(overridesDir, 'dynamodb.json'),
    );

    // DynamoDB alarms — one set of alarms per table in the inventory.
    if (inventory.resources.dynamoDBTables.length > 0) {
      new CommonDynamoDBAlarms(this, 'DynamoDBAlarms', {
        tables: inventory.resources.dynamoDBTables,
        alarmTopic: this.snsNotification.topic,
        overrides: dynamoDBOverrides,
        environment,
        region: this.region,
      });
    }
  }
}
