import * as cdk from 'aws-cdk-lib';
import { Construct } from 'constructs';
import { SnsNotification } from '../constructs/sns-notification.js';

export interface SnsStackProps extends cdk.StackProps {
  readonly environment: string;
  readonly alarmEmail: string;
}

/**
 * Dedicated stack for the shared SNS alarm topic.
 * Deployed once per environment. All alarm stacks reference this topic by ARN.
 */
export class SnsStack extends cdk.Stack {
  public readonly topicArn: string;

  constructor(scope: Construct, id: string, props: SnsStackProps) {
    super(scope, id, props);

    const snsNotification = new SnsNotification(this, 'SnsNotification', {
      environment: props.environment,
      alarmEmail: props.alarmEmail,
    });

    this.topicArn = snsNotification.topic.topicArn;
  }
}
