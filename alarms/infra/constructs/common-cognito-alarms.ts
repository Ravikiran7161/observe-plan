import { Construct } from 'constructs';
import * as cdk from 'aws-cdk-lib';
import * as cloudwatch from 'aws-cdk-lib/aws-cloudwatch';
import * as cloudwatchActions from 'aws-cdk-lib/aws-cloudwatch-actions';
import * as sns from 'aws-cdk-lib/aws-sns';
import { CognitoOverrides, AlarmThreshold } from '../alarm-config.js';

// ---------------------------------------------------------------------------
// Hardcoded fallback defaults.
// Precedence: per-pool override > defaults block in file > these values.
// ---------------------------------------------------------------------------
const FALLBACK = {
  throttleCount:  { threshold: 10, evaluationPeriods: 1, datapointsToAlarm: 1, periodMinutes: 5 },
  signInThrottles: { threshold: 5,  evaluationPeriods: 1, datapointsToAlarm: 1, periodMinutes: 5 },
} as const;

export interface CognitoUserPoolEntry {
  readonly userPoolId: string;
  readonly clientIds: string[];
}

export interface CommonCognitoAlarmsProps {
  /** Cognito user pool entries from the resource inventory. */
  readonly pools: CognitoUserPoolEntry[];
  /** SNS topic that all alarms publish to. */
  readonly alarmTopic: sns.ITopic;
  /** Optional overrides loaded from overrides/cognito.json. */
  readonly overrides?: CognitoOverrides;
  /** Environment name — used in alarm naming (e.g. 'main'). */
  readonly environment: string;
  /** AWS region abbreviation — used in alarm naming (e.g. 'use2'). */
  readonly region: string;
}

/**
 * Creates CloudWatch alarms for every Cognito user pool in the inventory.
 *
 * Alarms per pool (namespace: AWS/Cognito, dimension: UserPool):
 *   - ThrottleCount   — total throttled API calls to the pool (Sum)
 *   - SignInThrottles — throttled sign-in attempts, brute force indicator (Sum)
 */
export class CommonCognitoAlarms extends Construct {
  constructor(scope: Construct, id: string, props: CommonCognitoAlarmsProps) {
    super(scope, id);

    const { pools, alarmTopic, overrides, environment, region } = props;
    const action = new cloudwatchActions.SnsAction(alarmTopic);

    // Prefix: op-{region}-{environment}-alarms-cognito
    const prefix = `op-${region}-${environment}-alarms-cognito`;

    for (const pool of pools) {
      const perPool = overrides?.pools?.[pool.userPoolId];
      const defaults = overrides?.defaults;

      // Sanitise pool ID for alarm names and CDK logical IDs.
      const safeName = pool.userPoolId.toLowerCase().replace(/[^a-z0-9-]/g, '-');
      const safeId = safeName;

      const dimensionsMap = { UserPool: pool.userPoolId };

      // --- ThrottleCount ---
      const cfgThr = resolve(perPool?.throttleCount, defaults?.throttleCount, FALLBACK.throttleCount);
      const alarmThr = new cloudwatch.Alarm(this, `${safeId}-throttle-count`, {
        alarmName: `${prefix}-${safeName}-throttle-count`,
        alarmDescription: `Throttle count exceeded ${cfgThr.threshold} for Cognito pool ${pool.userPoolId}`,
        metric: new cloudwatch.Metric({
          namespace: 'AWS/Cognito',
          metricName: 'ThrottleCount',
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

      // --- SignInThrottles ---
      const cfgSit = resolve(perPool?.signInThrottles, defaults?.signInThrottles, FALLBACK.signInThrottles);
      const alarmSit = new cloudwatch.Alarm(this, `${safeId}-signin-throttles`, {
        alarmName: `${prefix}-${safeName}-signin-throttles`,
        alarmDescription: `Sign-in throttles exceeded ${cfgSit.threshold} for Cognito pool ${pool.userPoolId} — possible brute force`,
        metric: new cloudwatch.Metric({
          namespace: 'AWS/Cognito',
          metricName: 'SignInThrottles',
          dimensionsMap,
          statistic: 'Sum',
          period: cdk.Duration.minutes(cfgSit.periodMinutes),
        }),
        threshold: cfgSit.threshold,
        evaluationPeriods: cfgSit.evaluationPeriods,
        datapointsToAlarm: cfgSit.datapointsToAlarm,
        comparisonOperator: cloudwatch.ComparisonOperator.GREATER_THAN_THRESHOLD,
        treatMissingData: cloudwatch.TreatMissingData.NOT_BREACHING,
      });
      alarmSit.addAlarmAction(action);
      alarmSit.addOkAction(action);
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
