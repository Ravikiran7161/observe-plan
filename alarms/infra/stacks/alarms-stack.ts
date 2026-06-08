import * as cdk from 'aws-cdk-lib';
import * as path from 'path';
import * as sns from 'aws-cdk-lib/aws-sns';
import { fileURLToPath } from 'url';
import { Construct } from 'constructs';
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
const overridesDir = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../overrides');

export interface AlarmsStackProps extends cdk.StackProps {
  /** Environment name (e.g. 'main', 'dev'). */
  readonly environment: string;
  /** ARN of the shared SNS topic (created by the SNS stack). */
  readonly topicArn: string;
  /** Resource inventory produced by the discover CLI. */
  readonly inventory: ResourceInventory;
}

/**
 * CloudFormation stack for CloudWatch alarms in one scope.
 * References the shared SNS topic by ARN — does not create its own.
 */
export class AlarmsStack extends cdk.Stack {
  constructor(scope: Construct, id: string, props: AlarmsStackProps) {
    super(scope, id, props);

    const { environment, topicArn, inventory } = props;

    // Null-safe resource access
    const res = inventory.resources ?? ({} as any);
    const resources = {
      lambdas: res.lambdas ?? [],
      apiGateways: res.apiGateways ?? [],
      dynamoDBTables: res.dynamoDBTables ?? [],
      sqsQueues: res.sqsQueues ?? [],
      ecsClusters: res.ecsClusters ?? [],
      eventBuses: res.eventBuses ?? [],
      s3Buckets: res.s3Buckets ?? [],
      amplifyApps: res.amplifyApps ?? [],
      cognitoUserPools: res.cognitoUserPools ?? [],
    };

    // Import the shared SNS topic by ARN.
    const alarmTopic = sns.Topic.fromTopicArn(this, 'AlarmTopic', topicArn);

    // Load optional API Gateway overrides file.
    const apiGatewayOverrides: ApiGatewayOverrides | undefined = loadApiGatewayOverrides(
      path.join(overridesDir, 'apigateway.json'),
    );

    if (resources.apiGateways.length > 0) {
      new CommonApiGatewayAlarms(this, 'ApiGatewayAlarms', {
        apis: resources.apiGateways,
        alarmTopic,
        overrides: apiGatewayOverrides,
        environment,
        region: this.region,
      });
    }

    // Load optional Amplify overrides file.
    const amplifyOverrides: AmplifyOverrides | undefined = loadAmplifyOverrides(
      path.join(overridesDir, 'amplify.json'),
    );

    if (resources.amplifyApps.length > 0) {
      new CommonAmplifyAlarms(this, 'AmplifyAlarms', {
        apps: resources.amplifyApps,
        alarmTopic,
        overrides: amplifyOverrides,
        environment,
        region: this.region,
      });
    }

    // Load optional Lambda overrides file.
    const lambdaOverrides: LambdaOverrides | undefined = loadLambdaOverrides(
      path.join(overridesDir, 'lambda.json'),
    );

    if (resources.lambdas.length > 0) {
      new CommonLambdaAlarms(this, 'LambdaAlarms', {
        functions: resources.lambdas,
        alarmTopic,
        overrides: lambdaOverrides,
        environment,
        region: this.region,
      });
    }

    // Load optional Cognito overrides file.
    const cognitoOverrides: CognitoOverrides | undefined = loadCognitoOverrides(
      path.join(overridesDir, 'cognito.json'),
    );

    if (resources.cognitoUserPools.length > 0) {
      new CommonCognitoAlarms(this, 'CognitoAlarms', {
        pools: resources.cognitoUserPools,
        alarmTopic,
        overrides: cognitoOverrides,
        environment,
        region: this.region,
      });
    }

    // Load optional DynamoDB overrides file.
    const dynamoDBOverrides: DynamoDBOverrides | undefined = loadDynamoDBOverrides(
      path.join(overridesDir, 'dynamodb.json'),
    );

    if (resources.dynamoDBTables.length > 0) {
      new CommonDynamoDBAlarms(this, 'DynamoDBAlarms', {
        tables: resources.dynamoDBTables,
        alarmTopic,
        overrides: dynamoDBOverrides,
        environment,
        region: this.region,
      });
    }
  }
}
