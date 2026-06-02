import { Construct } from 'constructs';
import * as cdk from 'aws-cdk-lib';
import * as cloudwatch from 'aws-cdk-lib/aws-cloudwatch';
import * as cloudwatchActions from 'aws-cdk-lib/aws-cloudwatch-actions';
import * as sns from 'aws-cdk-lib/aws-sns';
import { ApiGatewayOverrides, AlarmThreshold } from '../alarm-config.js';

// ---------------------------------------------------------------------------
// Hardcoded fallback defaults — last resort if no override file is present.
// Precedence: per-api override > defaults block in file > these values.
// ---------------------------------------------------------------------------
const FALLBACK: Required<Record<'4xx' | '5xx' | 'latencyP95', Required<AlarmThreshold>>> = {
  '4xx':       { threshold: 150,  evaluationPeriods: 1, datapointsToAlarm: 1, periodMinutes: 5 },
  '5xx':       { threshold: 30,   evaluationPeriods: 1, datapointsToAlarm: 1, periodMinutes: 5 },
  latencyP95:  { threshold: 3000, evaluationPeriods: 2, datapointsToAlarm: 2, periodMinutes: 5 },
};

export interface ApiGatewayAlarmEntry {
  readonly kind: string;
  readonly apiName: string;
  readonly stage: string;
}

export interface CommonApiGatewayAlarmsProps {
  /** API Gateway entries from the resource inventory. */
  readonly apis: ApiGatewayAlarmEntry[];
  /** SNS topic that all alarms publish to. */
  readonly alarmTopic: sns.ITopic;
  /** Optional overrides loaded from overrides/apigateway.json. */
  readonly overrides?: ApiGatewayOverrides;
  /** Environment name — used in alarm naming (e.g. 'main'). */
  readonly environment: string;
  /** AWS region abbreviation — used in alarm naming (e.g. 'use2'). */
  readonly region: string;
}

export class CommonApiGatewayAlarms extends Construct {
  constructor(scope: Construct, id: string, props: CommonApiGatewayAlarmsProps) {
    super(scope, id);

    const { apis, alarmTopic, overrides, environment, region } = props;
    const action = new cloudwatchActions.SnsAction(alarmTopic);

    // Prefix shared by all alarm names in this construct.
    // Pattern: op-{region}-{environment}-alarms-apigateway
    const prefix = `op-${region}-${environment}-alarms-apigateway`;

    for (const api of apis) {
      const key = `${api.apiName}:${api.stage}`;
      const perApi = overrides?.apis?.[key];
      const defaults = overrides?.defaults;

      // Sanitise api name + stage for use in names (lowercase, hyphens only).
      const safeName = `${api.apiName}-${api.stage}`.toLowerCase().replace(/[^a-z0-9-]/g, '-');

      // CDK logical ID — must be unique within the construct, no special chars.
      const safeId = safeName;

      // --- 4xx ---
      const cfg4xx = resolve('4xx', perApi?.['4xx'], defaults?.['4xx'], FALLBACK['4xx']);
      const alarm4xx = new cloudwatch.Alarm(this, `${safeId}-4xx`, {
        alarmName: `${prefix}-${safeName}-4xx`,
        alarmDescription: `4xx error count exceeded ${cfg4xx.threshold} for ${key}`,
        metric: new cloudwatch.Metric({
          namespace: 'AWS/ApiGateway',
          metricName: '4XXError',
          dimensionsMap: { ApiName: api.apiName, Stage: api.stage },
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

      // --- 5xx ---
      const cfg5xx = resolve('5xx', perApi?.['5xx'], defaults?.['5xx'], FALLBACK['5xx']);
      const alarm5xx = new cloudwatch.Alarm(this, `${safeId}-5xx`, {
        alarmName: `${prefix}-${safeName}-5xx`,
        alarmDescription: `5xx error count exceeded ${cfg5xx.threshold} for ${key}`,
        metric: new cloudwatch.Metric({
          namespace: 'AWS/ApiGateway',
          metricName: '5XXError',
          dimensionsMap: { ApiName: api.apiName, Stage: api.stage },
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

      // --- Latency p95 ---
      const cfgLat = resolve('latencyP95', perApi?.latencyP95, defaults?.latencyP95, FALLBACK.latencyP95);
      const alarmLat = new cloudwatch.Alarm(this, `${safeId}-latency-p95`, {
        alarmName: `${prefix}-${safeName}-latency-p95`,
        alarmDescription: `p95 latency exceeded ${cfgLat.threshold}ms for ${key}`,
        metric: new cloudwatch.Metric({
          namespace: 'AWS/ApiGateway',
          metricName: 'Latency',
          dimensionsMap: { ApiName: api.apiName, Stage: api.stage },
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
    }
  }
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

type MetricKey = '4xx' | '5xx' | 'latencyP95';

/** Merges per-resource override → defaults block → hardcoded fallback. */
function resolve(
  _key: MetricKey,
  perResource: AlarmThreshold | undefined,
  defaults: AlarmThreshold | undefined,
  fallback: Required<AlarmThreshold>,
): Required<AlarmThreshold> {
  return {
    threshold:        perResource?.threshold        ?? defaults?.threshold        ?? fallback.threshold,
    evaluationPeriods: perResource?.evaluationPeriods ?? defaults?.evaluationPeriods ?? fallback.evaluationPeriods,
    datapointsToAlarm: perResource?.datapointsToAlarm ?? defaults?.datapointsToAlarm ?? fallback.datapointsToAlarm,
    periodMinutes:    perResource?.periodMinutes    ?? defaults?.periodMinutes    ?? fallback.periodMinutes,
  };
}
