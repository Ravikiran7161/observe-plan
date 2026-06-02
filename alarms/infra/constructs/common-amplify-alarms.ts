import { Construct } from 'constructs';
import * as cdk from 'aws-cdk-lib';
import * as cloudwatch from 'aws-cdk-lib/aws-cloudwatch';
import * as cloudwatchActions from 'aws-cdk-lib/aws-cloudwatch-actions';
import * as sns from 'aws-cdk-lib/aws-sns';
import { AmplifyOverrides, AlarmThreshold } from '../alarm-config.js';

// Hardcoded fallback defaults.
// Precedence: per-app override > defaults block in file > these values.
const FALLBACK = {
  '4xx':          { threshold: 100,   evaluationPeriods: 1, datapointsToAlarm: 1, periodMinutes: 5 },
  '5xx':          { threshold: 20,    evaluationPeriods: 1, datapointsToAlarm: 1, periodMinutes: 5 },
  latencyP95:     { threshold: 2,     evaluationPeriods: 2, datapointsToAlarm: 2, periodMinutes: 5 },
  requests:       { threshold: 10000, evaluationPeriods: 1, datapointsToAlarm: 1, periodMinutes: 5 },
  tokensConsumed: { threshold: 1000,  evaluationPeriods: 2, datapointsToAlarm: 2, periodMinutes: 5 },
} as const;

export interface AmplifyAppEntry {
  readonly appId: string;
  readonly name: string;
}

export interface CommonAmplifyAlarmsProps {
  /** Amplify app entries from the resource inventory. */
  readonly apps: AmplifyAppEntry[];
  /** SNS topic that all alarms publish to. */
  readonly alarmTopic: sns.ITopic;
  /** Optional overrides loaded from overrides/amplify.json. */
  readonly overrides?: AmplifyOverrides;
  /** Environment name — used in alarm naming (e.g. 'main'). */
  readonly environment: string;
  /** AWS region abbreviation — used in alarm naming (e.g. 'use2'). */
  readonly region: string;
}

export class CommonAmplifyAlarms extends Construct {
  constructor(scope: Construct, id: string, props: CommonAmplifyAlarmsProps) {
    super(scope, id);

    const { apps, alarmTopic, overrides, environment, region } = props;
    const action = new cloudwatchActions.SnsAction(alarmTopic);

    // Prefix shared by all alarm names in this construct.
    // Pattern: op-{region}-{environment}-alarms-amplify
    const prefix = `op-${region}-${environment}-alarms-amplify`;

    for (const app of apps) {
      const perApp = overrides?.apps?.[app.name];
      const defaults = overrides?.defaults;

      // Sanitise app name for use in alarm names and CDK logical IDs.
      const safeName = app.name.toLowerCase().replace(/[^a-z0-9-]/g, '-');
      const safeId = safeName;

      // Shared dimension — Amplify uses App ID (not name) as the dimension value.
      const dimensionsMap = { App: app.appId };

      // --- 4xx errors ---
      const cfg4xx = resolve(perApp?.['4xx'], defaults?.['4xx'], FALLBACK['4xx']);
      const alarm4xx = new cloudwatch.Alarm(this, `${safeId}-4xx`, {
        alarmName: `${prefix}-${safeName}-4xx`,
        alarmDescription: `4xx error count exceeded ${cfg4xx.threshold} for Amplify app ${app.name}`,
        metric: new cloudwatch.Metric({
          namespace: 'AWS/AmplifyHosting',
          metricName: '4xxErrors',
          dimensionsMap,
          statistic: 'Sum',
          period: cdk.Duration.minutes(cfg4xx.periodMinutes),
        }),
        threshold: cfg4xx.threshold,
        evaluationPeriods: cfg4xx.evaluationPeriods,
        datapointsToAlarm: cfg4xx.datapointsToAlarm,
        comparisonOperator: cloudwatch.ComparisonOperator.GREATER_THAN_THRESHOLD,
        treatMissingData: cloudwatch.TreatMissingData.NOT_BREACHING,
      });
      alarm4xx.addAlarmAction(action);
      alarm4xx.addOkAction(action);

      // --- 5xx errors ---
      const cfg5xx = resolve(perApp?.['5xx'], defaults?.['5xx'], FALLBACK['5xx']);
      const alarm5xx = new cloudwatch.Alarm(this, `${safeId}-5xx`, {
        alarmName: `${prefix}-${safeName}-5xx`,
        alarmDescription: `5xx error count exceeded ${cfg5xx.threshold} for Amplify app ${app.name}`,
        metric: new cloudwatch.Metric({
          namespace: 'AWS/AmplifyHosting',
          metricName: '5xxErrors',
          dimensionsMap,
          statistic: 'Sum',
          period: cdk.Duration.minutes(cfg5xx.periodMinutes),
        }),
        threshold: cfg5xx.threshold,
        evaluationPeriods: cfg5xx.evaluationPeriods,
        datapointsToAlarm: cfg5xx.datapointsToAlarm,
        comparisonOperator: cloudwatch.ComparisonOperator.GREATER_THAN_THRESHOLD,
        treatMissingData: cloudwatch.TreatMissingData.NOT_BREACHING,
      });
      alarm5xx.addAlarmAction(action);
      alarm5xx.addOkAction(action);

      // --- Latency p95 (TTFB in seconds) ---
      const cfgLat = resolve(perApp?.latencyP95, defaults?.latencyP95, FALLBACK.latencyP95);
      const alarmLat = new cloudwatch.Alarm(this, `${safeId}-latency-p95`, {
        alarmName: `${prefix}-${safeName}-latency-p95`,
        alarmDescription: `p95 TTFB exceeded ${cfgLat.threshold}s for Amplify app ${app.name}`,
        metric: new cloudwatch.Metric({
          namespace: 'AWS/AmplifyHosting',
          metricName: 'Latency',
          dimensionsMap,
          statistic: 'p95',
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

      // --- Requests (traffic spike) ---
      const cfgReq = resolve(perApp?.requests, defaults?.requests, FALLBACK.requests);
      const alarmReq = new cloudwatch.Alarm(this, `${safeId}-requests`, {
        alarmName: `${prefix}-${safeName}-requests`,
        alarmDescription: `Request count exceeded ${cfgReq.threshold} for Amplify app ${app.name}`,
        metric: new cloudwatch.Metric({
          namespace: 'AWS/AmplifyHosting',
          metricName: 'Requests',
          dimensionsMap,
          statistic: 'Sum',
          period: cdk.Duration.minutes(cfgReq.periodMinutes),
        }),
        threshold: cfgReq.threshold,
        evaluationPeriods: cfgReq.evaluationPeriods,
        datapointsToAlarm: cfgReq.datapointsToAlarm,
        comparisonOperator: cloudwatch.ComparisonOperator.GREATER_THAN_THRESHOLD,
        treatMissingData: cloudwatch.TreatMissingData.NOT_BREACHING,
      });
      alarmReq.addAlarmAction(action);
      alarmReq.addOkAction(action);

      // --- TokensConsumed (throttling risk) ---
      const cfgTok = resolve(perApp?.tokensConsumed, defaults?.tokensConsumed, FALLBACK.tokensConsumed);
      const alarmTok = new cloudwatch.Alarm(this, `${safeId}-tokens`, {
        alarmName: `${prefix}-${safeName}-tokens-consumed`,
        alarmDescription: `Token consumption exceeded ${cfgTok.threshold} for Amplify app ${app.name} — throttling risk`,
        metric: new cloudwatch.Metric({
          namespace: 'AWS/AmplifyHosting',
          metricName: 'TokensConsumed',
          dimensionsMap,
          statistic: 'Sum',
          period: cdk.Duration.minutes(cfgTok.periodMinutes),
        }),
        threshold: cfgTok.threshold,
        evaluationPeriods: cfgTok.evaluationPeriods,
        datapointsToAlarm: cfgTok.datapointsToAlarm,
        comparisonOperator: cloudwatch.ComparisonOperator.GREATER_THAN_THRESHOLD,
        treatMissingData: cloudwatch.TreatMissingData.NOT_BREACHING,
      });
      alarmTok.addAlarmAction(action);
      alarmTok.addOkAction(action);
    }
  }
}

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

/** Merges per-resource override → defaults block → hardcoded fallback. */
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
