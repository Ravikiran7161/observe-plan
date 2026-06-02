import { Construct } from 'constructs';
import * as cdk from 'aws-cdk-lib';
import * as cloudwatch from 'aws-cdk-lib/aws-cloudwatch';
import * as cloudwatchActions from 'aws-cdk-lib/aws-cloudwatch-actions';
import * as sns from 'aws-cdk-lib/aws-sns';
import { LambdaOverrides, AlarmThreshold } from '../alarm-config.js';

// ---------------------------------------------------------------------------
// Hardcoded fallback defaults.
// Precedence: per-function override > defaults block in file > these values.
// ---------------------------------------------------------------------------
const FALLBACK = {
  errors:               { threshold: 5,     evaluationPeriods: 1, datapointsToAlarm: 1, periodMinutes: 5 },
  throttles:            { threshold: 1,     evaluationPeriods: 1, datapointsToAlarm: 1, periodMinutes: 5 },
  durationP95:          { threshold: 10000, evaluationPeriods: 2, datapointsToAlarm: 2, periodMinutes: 5 },
  concurrentExecutions: { threshold: 500,   evaluationPeriods: 1, datapointsToAlarm: 1, periodMinutes: 5 },
} as const;

export interface LambdaFunctionEntry {
  readonly functionName: string;
}

export interface CommonLambdaAlarmsProps {
  /** Lambda function entries from the resource inventory. */
  readonly functions: LambdaFunctionEntry[];
  /** SNS topic that all alarms publish to. */
  readonly alarmTopic: sns.ITopic;
  /** Optional overrides loaded from overrides/lambda.json. */
  readonly overrides?: LambdaOverrides;
  /** Environment name — used in alarm naming (e.g. 'main'). */
  readonly environment: string;
  /** AWS region abbreviation — used in alarm naming (e.g. 'use2'). */
  readonly region: string;
}

export class CommonLambdaAlarms extends Construct {
  constructor(scope: Construct, id: string, props: CommonLambdaAlarmsProps) {
    super(scope, id);

    const { functions, alarmTopic, overrides, environment, region } = props;
    const action = new cloudwatchActions.SnsAction(alarmTopic);

    // Prefix: op-{region}-{environment}-alarms-lambda
    const prefix = `op-${region}-${environment}-alarms-lambda`;

    for (const fn of functions) {
      const perFn = overrides?.functions?.[fn.functionName];
      const defaults = overrides?.defaults;

      // Sanitise function name for alarm names and CDK logical IDs.
      const safeName = fn.functionName.toLowerCase().replace(/[^a-z0-9-]/g, '-');
      const safeId = safeName;

      const dimensionsMap = { FunctionName: fn.functionName };

      // --- Errors ---
      const cfgErr = resolve(perFn?.errors, defaults?.errors, FALLBACK.errors);
      const alarmErr = new cloudwatch.Alarm(this, `${safeId}-errors`, {
        alarmName: `${prefix}-${safeName}-errors`,
        alarmDescription: `Error count exceeded ${cfgErr.threshold} for Lambda ${fn.functionName}`,
        metric: new cloudwatch.Metric({
          namespace: 'AWS/Lambda',
          metricName: 'Errors',
          dimensionsMap,
          statistic: 'Sum',
          period: cdk.Duration.minutes(cfgErr.periodMinutes),
        }),
        threshold: cfgErr.threshold,
        evaluationPeriods: cfgErr.evaluationPeriods,
        datapointsToAlarm: cfgErr.datapointsToAlarm,
        comparisonOperator: cloudwatch.ComparisonOperator.GREATER_THAN_THRESHOLD,
        treatMissingData: cloudwatch.TreatMissingData.NOT_BREACHING,
      });
      alarmErr.addAlarmAction(action);
      alarmErr.addOkAction(action);

      // --- Throttles ---
      const cfgThr = resolve(perFn?.throttles, defaults?.throttles, FALLBACK.throttles);
      const alarmThr = new cloudwatch.Alarm(this, `${safeId}-throttles`, {
        alarmName: `${prefix}-${safeName}-throttles`,
        alarmDescription: `Throttle count exceeded ${cfgThr.threshold} for Lambda ${fn.functionName}`,
        metric: new cloudwatch.Metric({
          namespace: 'AWS/Lambda',
          metricName: 'Throttles',
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

      // --- Duration p95 (ms) ---
      const cfgDur = resolve(perFn?.durationP95, defaults?.durationP95, FALLBACK.durationP95);
      const alarmDur = new cloudwatch.Alarm(this, `${safeId}-duration-p95`, {
        alarmName: `${prefix}-${safeName}-duration-p95`,
        alarmDescription: `p95 duration exceeded ${cfgDur.threshold}ms for Lambda ${fn.functionName}`,
        metric: new cloudwatch.Metric({
          namespace: 'AWS/Lambda',
          metricName: 'Duration',
          dimensionsMap,
          statistic: 'p95',
          period: cdk.Duration.minutes(cfgDur.periodMinutes),
        }),
        threshold: cfgDur.threshold,
        evaluationPeriods: cfgDur.evaluationPeriods,
        datapointsToAlarm: cfgDur.datapointsToAlarm,
        comparisonOperator: cloudwatch.ComparisonOperator.GREATER_THAN_THRESHOLD,
        treatMissingData: cloudwatch.TreatMissingData.NOT_BREACHING,
      });
      alarmDur.addAlarmAction(action);
      alarmDur.addOkAction(action);

      // --- ConcurrentExecutions (Max) ---
      const cfgCon = resolve(perFn?.concurrentExecutions, defaults?.concurrentExecutions, FALLBACK.concurrentExecutions);
      const alarmCon = new cloudwatch.Alarm(this, `${safeId}-concurrent`, {
        alarmName: `${prefix}-${safeName}-concurrent-executions`,
        alarmDescription: `Concurrent executions exceeded ${cfgCon.threshold} for Lambda ${fn.functionName}`,
        metric: new cloudwatch.Metric({
          namespace: 'AWS/Lambda',
          metricName: 'ConcurrentExecutions',
          dimensionsMap,
          statistic: 'Maximum',
          period: cdk.Duration.minutes(cfgCon.periodMinutes),
        }),
        threshold: cfgCon.threshold,
        evaluationPeriods: cfgCon.evaluationPeriods,
        datapointsToAlarm: cfgCon.datapointsToAlarm,
        comparisonOperator: cloudwatch.ComparisonOperator.GREATER_THAN_THRESHOLD,
        treatMissingData: cloudwatch.TreatMissingData.NOT_BREACHING,
      });
      alarmCon.addAlarmAction(action);
      alarmCon.addOkAction(action);
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
