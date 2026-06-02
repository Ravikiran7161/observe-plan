import * as fs from 'fs';
import * as path from 'path';

/**
 * Schema of the per-environment config file (e.g. config/main.json).
 * The discover CLI reads only `tags`; the CDK stack reads the full config.
 */
export interface AlarmEnvConfig {
  /** Tag filters used to scope resource discovery (e.g. { environment: 'main' }). */
  readonly tags: Record<string, string>;
  /** Email address to subscribe to the SNS alarm topic. */
  readonly alarmEmail: string;
}

/**
 * Schema of the generated resource inventory (generated/resources.json).
 * Written by the discover CLI, read by the CDK stack at synth time.
 */
export interface ResourceInventory {
  readonly tags: Record<string, string>;
  readonly generatedAt: string;
  readonly resources: {
    readonly lambdas: Array<{ functionName: string }>;
    readonly apiGateways: Array<{ kind: string; apiName: string; stage: string }>;
    readonly dynamoDBTables: Array<{ tableName: string }>;
    readonly sqsQueues: Array<{ queueName: string }>;
    readonly ecsClusters: Array<{ clusterName: string }>;
    readonly eventBuses: Array<{ eventBusName: string }>;
    readonly s3Buckets: Array<{ bucketName: string }>;
    readonly amplifyApps: Array<{ appId: string; name: string }>;
    readonly cognitoUserPools: Array<{ userPoolId: string; clientIds: string[] }>;
  };
}

// ---------------------------------------------------------------------------
// Override types
// ---------------------------------------------------------------------------

/** Threshold settings for a single alarm metric. All fields are optional —
 *  missing fields fall back to the defaults block, then to construct hardcoded values. */
export interface AlarmThreshold {
  readonly threshold?: number;
  readonly evaluationPeriods?: number;
  readonly datapointsToAlarm?: number;
  readonly periodMinutes?: number;
}

/** Schema of overrides/dynamodb.json */
export interface DynamoDBOverrides {
  readonly defaults?: {
    readonly systemErrors?: AlarmThreshold;
    readonly throttledRequests?: AlarmThreshold;
    readonly latencyP99?: AlarmThreshold;
  };
  /** Key format: table name (e.g. "cp-use2-main-tenant-mgmt-tenants") */
  readonly tables?: Record<string, {
    readonly systemErrors?: AlarmThreshold;
    readonly throttledRequests?: AlarmThreshold;
    readonly latencyP99?: AlarmThreshold;
  }>;
}

/** Loads the optional DynamoDB override file.
 *  Returns undefined if the file does not exist. */
export function loadDynamoDBOverrides(overridesPath: string): DynamoDBOverrides | undefined {
  if (!fs.existsSync(overridesPath)) {
    return undefined;
  }
  return JSON.parse(fs.readFileSync(overridesPath, 'utf-8')) as DynamoDBOverrides;
}

/** Schema of overrides/lambda.json */
export interface LambdaOverrides {
  readonly defaults?: {
    readonly errors?: AlarmThreshold;
    readonly throttles?: AlarmThreshold;
    readonly durationP95?: AlarmThreshold;
    readonly concurrentExecutions?: AlarmThreshold;
  };
  /** Key format: function name (e.g. "op-main-metrics-dashboard-fn") */
  readonly functions?: Record<string, {
    readonly errors?: AlarmThreshold;
    readonly throttles?: AlarmThreshold;
    readonly durationP95?: AlarmThreshold;
    readonly concurrentExecutions?: AlarmThreshold;
  }>;
}

/** Loads the optional Lambda override file.
 *  Returns undefined if the file does not exist. */
export function loadLambdaOverrides(overridesPath: string): LambdaOverrides | undefined {
  if (!fs.existsSync(overridesPath)) {
    return undefined;
  }
  return JSON.parse(fs.readFileSync(overridesPath, 'utf-8')) as LambdaOverrides;
}

/** Schema of overrides/cognito.json */
export interface CognitoOverrides {
  readonly defaults?: {
    readonly throttleCount?: AlarmThreshold;
    readonly signInThrottles?: AlarmThreshold;
  };
  /** Key format: user pool ID (e.g. "us-east-2_abc123") */
  readonly pools?: Record<string, {
    readonly throttleCount?: AlarmThreshold;
    readonly signInThrottles?: AlarmThreshold;
  }>;
}

/** Loads the optional Cognito override file.
 *  Returns undefined if the file does not exist. */
export function loadCognitoOverrides(overridesPath: string): CognitoOverrides | undefined {
  if (!fs.existsSync(overridesPath)) {
    return undefined;
  }
  return JSON.parse(fs.readFileSync(overridesPath, 'utf-8')) as CognitoOverrides;
}

/** Schema of overrides/amplify.json */
export interface AmplifyOverrides {
  readonly defaults?: {
    readonly '4xx'?: AlarmThreshold;
    readonly '5xx'?: AlarmThreshold;
    readonly latencyP95?: AlarmThreshold;
    readonly requests?: AlarmThreshold;
    readonly tokensConsumed?: AlarmThreshold;
  };
  /** Key format: app name (e.g. "my-amplify-app") */
  readonly apps?: Record<string, {
    readonly '4xx'?: AlarmThreshold;
    readonly '5xx'?: AlarmThreshold;
    readonly latencyP95?: AlarmThreshold;
    readonly requests?: AlarmThreshold;
    readonly tokensConsumed?: AlarmThreshold;
  }>;
}

/** Loads the optional Amplify override file.
 *  Returns undefined if the file does not exist. */
export function loadAmplifyOverrides(overridesPath: string): AmplifyOverrides | undefined {
  if (!fs.existsSync(overridesPath)) {
    return undefined;
  }
  return JSON.parse(fs.readFileSync(overridesPath, 'utf-8')) as AmplifyOverrides;
}

/** Schema of overrides/apigateway.json */
export interface ApiGatewayOverrides {
  readonly defaults?: {
    readonly '4xx'?: AlarmThreshold;
    readonly '5xx'?: AlarmThreshold;
    readonly latencyP95?: AlarmThreshold;
  };
  /** Key format: "apiName:stage" e.g. "orders-api:prod" */
  readonly apis?: Record<string, {
    readonly '4xx'?: AlarmThreshold;
    readonly '5xx'?: AlarmThreshold;
    readonly latencyP95?: AlarmThreshold;
  }>;
}

/** Loads the optional API Gateway override file.
 *  Returns undefined if the file does not exist. */
export function loadApiGatewayOverrides(overridesPath: string): ApiGatewayOverrides | undefined {
  if (!fs.existsSync(overridesPath)) {
    return undefined;
  }
  return JSON.parse(fs.readFileSync(overridesPath, 'utf-8')) as ApiGatewayOverrides;
}

/** Loads and validates the per-environment alarm config file. */
export function loadAlarmEnvConfig(configPath: string): AlarmEnvConfig {
  const resolved = path.resolve(configPath);
  if (!fs.existsSync(resolved)) {
    throw new Error(`Alarm config file not found: ${resolved}`);
  }
  const raw = JSON.parse(fs.readFileSync(resolved, 'utf-8')) as AlarmEnvConfig;
  if (!raw.tags || Object.keys(raw.tags).length === 0) {
    throw new Error(`Alarm config ${resolved} must contain at least one tag`);
  }
  if (!raw.alarmEmail) {
    throw new Error(`Alarm config ${resolved} must contain an alarmEmail`);
  }
  return raw;
}

/** Loads and validates the generated resource inventory file. */
export function loadResourceInventory(inventoryPath: string): ResourceInventory {
  const resolved = path.resolve(inventoryPath);
  if (!fs.existsSync(resolved)) {
    throw new Error(
      `Resource inventory not found: ${resolved}\n` +
      `Run 'task discover-alarm-resources config=<config.json>' first.`,
    );
  }
  return JSON.parse(fs.readFileSync(resolved, 'utf-8')) as ResourceInventory;
}

/** Returns an empty resource inventory. Used during destroy operations. */
export function emptyInventory(): ResourceInventory {
  return {
    tags: {},
    generatedAt: '',
    resources: {
      lambdas: [],
      apiGateways: [],
      dynamoDBTables: [],
      sqsQueues: [],
      ecsClusters: [],
      eventBuses: [],
      s3Buckets: [],
      amplifyApps: [],
      cognitoUserPools: [],
    },
  };
}

