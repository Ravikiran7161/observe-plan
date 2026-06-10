import * as cdk from 'aws-cdk-lib';
import * as path from 'path';
import * as sns from 'aws-cdk-lib/aws-sns';
import { fileURLToPath } from 'url';
import { Construct } from 'constructs';
import { CommonLogAlarms } from '../constructs/common-log-alarms.js';
import { LogAlarmsConfig, loadLogAlarmsConfig } from '../alarm-config.js';

const overridesDir = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../overrides');

export interface LogAlarmsStackProps extends cdk.StackProps {
  readonly environment: string;
  readonly topicArn: string;
}

/**
 * Separate stack for log-based alarms (metric filters on CloudWatch Logs).
 * Config-driven — reads overrides/log-apigateway.json (and future log-amplify.json etc).
 * One stack for all log alarms. Independent from the tag-discovered metric alarm stacks.
 */
export class LogAlarmsStack extends cdk.Stack {
  constructor(scope: Construct, id: string, props: LogAlarmsStackProps) {
    super(scope, id, props);

    const { environment, topicArn } = props;
    const alarmTopic = sns.Topic.fromTopicArn(this, 'AlarmTopic', topicArn);

    // Load API Gateway log alarms config.
    const apiGatewayLogConfig: LogAlarmsConfig | undefined = loadLogAlarmsConfig(
      path.join(overridesDir, 'log-apigateway.json'),
    );

    if (apiGatewayLogConfig && apiGatewayLogConfig.logAlarms.length > 0) {
      new CommonLogAlarms(this, 'ApiGatewayLogAlarms', {
        config: apiGatewayLogConfig,
        alarmTopic,
        environment,
        region: this.region,
      });
    }
  }
}
