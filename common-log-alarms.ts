import { Construct } from 'constructs';
import * as cdk from 'aws-cdk-lib';
import * as cloudwatch from 'aws-cdk-lib/aws-cloudwatch';
import * as cloudwatchActions from 'aws-cdk-lib/aws-cloudwatch-actions';
import * as logs from 'aws-cdk-lib/aws-logs';
import * as sns from 'aws-cdk-lib/aws-sns';
import { LogAlarmsConfig, LogFilterEntry } from '../alarm-config.js';

export interface CommonLogAlarmsProps {
  /** Log alarms config loaded from overrides/log-apigateway.json. */
  readonly config: LogAlarmsConfig;
  /** SNS topic that all alarms publish to. */
  readonly alarmTopic: sns.ITopic;
  /** Environment name — used in alarm naming (e.g. 'main'). */
  readonly environment: string;
  /** AWS region abbreviation — used in alarm naming (e.g. 'use2'). */
  readonly region: string;
}

/**
 * Creates CloudWatch Metric Filters + Alarms from log patterns.
 *
 * For each log group in the config, for each filter pattern:
 *   1. Creates a MetricFilter on the log group
 *   2. Creates an Alarm on the resulting custom metric
 *   3. Wires the alarm to the shared SNS topic
 *
 * Alarm naming: op-{region}-{env}-alarms-log-{filter-name}
 */
export class CommonLogAlarms extends Construct {
  constructor(scope: Construct, id: string, props: CommonLogAlarmsProps) {
    super(scope, id);

    const { config, alarmTopic, environment, region } = props;
    const action = new cloudwatchActions.SnsAction(alarmTopic);
    const prefix = `op-${region}-${environment}-alarms-log`;
    const metricNamespace = `ObservePlane/LogAlarms/${environment}`;

    for (const group of config.logAlarms) {
      // Import the existing log group by name (do not create it).
      const logGroupSafe = sanitize(group.logGroup);
      const logGroup = logs.LogGroup.fromLogGroupName(
        this,
        `LogGroup-${logGroupSafe}`,
        group.logGroup,
      );

      for (const filter of group.filters) {
        const safeName = sanitize(filter.name);
        const uniqueName = `${logGroupSafe}-${safeName}`;
        const metricName = uniqueName;
        const period = filter.periodMinutes ?? 5;
        const evaluationPeriods = filter.evaluationPeriods ?? 1;
        const datapointsToAlarm = filter.datapointsToAlarm ?? 1;

        // Metric Filter — transforms log pattern matches into a CloudWatch metric.
        new logs.MetricFilter(this, `MF-${uniqueName}`, {
          logGroup,
          filterPattern: logs.FilterPattern.literal(filter.pattern),
          metricNamespace,
          metricName,
          metricValue: '1',
          defaultValue: 0,
        });

        // Alarm on the custom metric.
        const alarm = new cloudwatch.Alarm(this, `Alarm-${uniqueName}`, {
          alarmName: `${prefix}-${logGroupSafe}-${safeName}`,
          alarmDescription: `Log pattern "${filter.pattern}" exceeded ${filter.threshold} in ${group.logGroup}`,
          metric: new cloudwatch.Metric({
            namespace: metricNamespace,
            metricName,
            statistic: 'Sum',
            period: cdk.Duration.minutes(period),
          }),
          threshold: filter.threshold,
          evaluationPeriods,
          datapointsToAlarm,
          comparisonOperator: cloudwatch.ComparisonOperator.GREATER_THAN_THRESHOLD,
          treatMissingData: cloudwatch.TreatMissingData.NOT_BREACHING,
        });
        alarm.addAlarmAction(action);
        alarm.addOkAction(action);
      }
    }
  }
}

function sanitize(name: string): string {
  return name.toLowerCase().replace(/[^a-z0-9-]/g, '-').replace(/-+/g, '-');
}
