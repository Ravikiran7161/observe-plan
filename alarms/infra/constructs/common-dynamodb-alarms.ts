import { Construct } from 'constructs';
import * as cdk from 'aws-cdk-lib';
import * as cloudwatch from 'aws-cdk-lib/aws-cloudwatch';
import * as cloudwatchActions from 'aws-cdk-lib/aws-cloudwatch-actions';
import * as sns from 'aws-cdk-lib/aws-sns';
import { DynamoDBOverrides, AlarmThreshold } from '../alarm-config.js';

// ---------------------------------------------------------------------------
// Hardcoded fallback defaults.
// Precedence: per-table override > defaults block in file > these values.
// ---------------------------------------------------------------------------
const FALLBACK = {
  systemErrors:       { threshold: 1,   evaluationPeriods: 1, datapointsToAlarm: 1, periodMinutes: 5 },
  throttledRequests:  { threshold: 10,  evaluationPeriods: 1, datapointsToAlarm: 1, periodMinutes: 5 },
  latencyP99:         { threshold: 100, evaluationPeriods: 2, datapointsToAlarm: 2, periodMinutes: 5 },
} as const;

export interface DynamoDBTableEntry {
  readonly tableName: string;
}

export interface CommonDynamoDBAlarmProps {
  /** DynamoDB table entries from the resource inventory. */
  readonly tables: DynamoDBTableEntry[];
  /** SNS topic that all alarms publish to. */
  readonly alarmTopic: sns.ITopic;
  /** Optional overrides loaded from overrides/dynamodb.json. */
  readonly overrides?: DynamoDBOverrides;
  /** Environment name — used in alarm naming (e.g. 'main'). */
  readonly environment: string;
  /** AWS region abbreviation — used in alarm naming (e.g. 'use2'). */
  readonly region: string;
}

/**
 * Creates CloudWatch alarms for every DynamoDB table in the inventory.
 *
 * Alarms per table (namespace: AWS/DynamoDB, dimension: TableName):
 *   - SystemErrors          — AWS-side 5xx errors (Sum)
 *   - ThrottledRequests     — capacity exceeded (Sum)
 *   - SuccessfulRequestLatency p99 — slow reads/writes in ms (p99)
 */
export class CommonDynamoDBAlarms extends Construct {
  constructor(scope: Construct, id: string, props: CommonDynamoDBAlarmProps) {
    super(scope, id);

    const { tables, alarmTopic, overrides, environment, region } = props;
    const action = new cloudwatchActions.SnsAction(alarmTopic);

    // Prefix: op-{region}-{environment}-alarms-dynamodb
    const prefix = `op-${region}-${environment}-alarms-dynamodb`;

    for (const table of tables) {
      const perTable = overrides?.tables?.[table.tableName];
      const defaults = overrides?.defaults;

      const safeName = table.tableName.toLowerCase().replace(/[^a-z0-9-]/g, '-');
      const safeId = safeName;
      const dimensionsMap = { TableName: table.tableName };

      // --- SystemErrors ---
      const cfgSys = resolve(perTable?.systemErrors, defaults?.systemErrors, FALLBACK.systemErrors);
      const alarmSys = new cloudwatch.Alarm(this, `${safeId}-system-errors`, {
        alarmName: `${prefix}-${safeName}-system-errors`,
        alarmDescription: `System errors exceeded ${cfgSys.threshold} for DynamoDB table ${table.tableName}`,
        metric: new cloudwatch.Metric({
          namespace: 'AWS/DynamoDB',
          metricName: 'SystemErrors',
          dimensionsMap,
          statistic: 'Sum',
          period: cdk.Duration.minutes(cfgSys.periodMinutes),
        }),
        threshold: cfgSys.threshold,
        evaluationPeriods: cfgSys.evaluationPeriods,
        datapointsToAlarm: cfgSys.datapointsToAlarm,
        comparisonOperator: cloudwatch.ComparisonOperator.GREATER_THAN_THRESHOLD,
        treatMissingData: cloudwatch.TreatMissingData.NOT_BREACHING,
      });
      alarmSys.addAlarmAction(action);
      alarmSys.addOkAction(action);

      // --- ThrottledRequests ---
      const cfgThr = resolve(perTable?.throttledRequests, defaults?.throttledRequests, FALLBACK.throttledRequests);
      const alarmThr = new cloudwatch.Alarm(this, `${safeId}-throttled-requests`, {
        alarmName: `${prefix}-${safeName}-throttled-requests`,
        alarmDescription: `Throttled requests exceeded ${cfgThr.threshold} for DynamoDB table ${table.tableName}`,
        metric: new cloudwatch.Metric({
          namespace: 'AWS/DynamoDB',
          metricName: 'ThrottledRequests',
          dimensionsMap,
          statistic: 'Sum',
          period: cdk.Duration.minutes(cfgThr.periodMinutes),
        }),
        threshold: cfgThr.threshold,
        evaluationPeriods: cfgThr.evaluationPeriods,
        datapointsToAlarm: cfgThr.datapointsToAlarm,
        comparisonOperator: cloudwatch.ComparisonOperator.GREATER_THAN_THRESHOLD,
        treatMissingData: cloudwatch.TreatMissingData.NOT_BREACHING,
      });
      alarmThr.addAlarmAction(action);
      alarmThr.addOkAction(action);

      // --- SuccessfulRequestLatency p99 (ms) ---
      const cfgLat = resolve(perTable?.latencyP99, defaults?.latencyP99, FALLBACK.latencyP99);
      const alarmLat = new cloudwatch.Alarm(this, `${safeId}-latency-p99`, {
        alarmName: `${prefix}-${safeName}-latency-p99`,
        alarmDescription: `p99 request latency exceeded ${cfgLat.threshold}ms for DynamoDB table ${table.tableName}`,
        metric: new cloudwatch.Metric({
          namespace: 'AWS/DynamoDB',
          metricName: 'SuccessfulRequestLatency',
          dimensionsMap,
          statistic: 'p99',
          period: cdk.Duration.minutes(cfgLat.periodMinutes),
        }),
        threshold: cfgLat.threshold,
        evaluationPeriods: cfgLat.evaluationPeriods,
        datapointsToAlarm: cfgLat.datapointsToAlarm,
        comparisonOperator: cloudwatch.ComparisonOperator.GREATER_THAN_THRESHOLD,
        treatMissingData: cloudwatch.TreatMissingData.NOT_BREACHING,
      });
      alarmLat.addAlarmAction(action);
      alarmLat.addOkAction(action);
    }
  }
}

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

function resolve(
  perResource: AlarmThreshold | undefined,
  defaults: AlarmThreshold | undefined,
  fallback: Required<AlarmThreshold>,
): Required<AlarmThreshold> {
  return {
    threshold:         perResource?.threshold         ?? defaults?.threshold         ?? fallback.threshold,
    evaluationPeriods: perResource?.evaluationPeriods ?? defaults?.evaluationPeriods ?? fallback.evaluationPeriods,
    datapointsToAlarm: perResource?.datapointsToAlarm ?? defaults?.datapointsToAlarm ?? fallback.datapointsToAlarm,
    periodMinutes:     perResource?.periodMinutes     ?? defaults?.periodMinutes     ?? fallback.periodMinutes,
  };
}
