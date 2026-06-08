import { Construct } from 'constructs';
import { Stack } from 'aws-cdk-lib';
import * as cdk from 'aws-cdk-lib';
import * as sns from 'aws-cdk-lib/aws-sns';
import * as subscriptions from 'aws-cdk-lib/aws-sns-subscriptions';
import { ResourceName, AwsService, Plane } from '@cmd-saas/libs-infra-resource';
import { ObservePlaneDomain } from '../domains.js';

export interface SnsNotificationProps {
  /** Environment name (e.g. 'main', 'dev'). Used for resource naming. */
  readonly environment: string;
  /** Email address to subscribe to the alarm topic. */
  readonly alarmEmail: string;
}

/**
 * Creates a single shared SNS topic for all CloudWatch alarms in this environment.
 * All alarm constructs in the stack receive the topic ARN via `topicArn`.
 * The email subscription requires manual confirmation after first deploy.
 */
export class SnsNotification extends Construct {
  /** The SNS topic that all alarms should publish to. */
  public readonly topic: sns.Topic;

  constructor(scope: Construct, id: string, props: SnsNotificationProps) {
    super(scope, id);

    const { environment, alarmEmail } = props;

    this.topic = new sns.Topic(this, 'AlarmTopic', {
      topicName: new ResourceName({
        plane: Plane.OP,
        environment,
        domain: ObservePlaneDomain.METRICS,
        purposeDescriptor: 'alarms',
        awsService: AwsService.SNS,
        scope: Stack.of(this),
      }).toString(),
      displayName: `Observe Plane alarms — ${environment}`,
    });

    // Email subscription, requires manual confirmation on first deploy.
    this.topic.addSubscription(
      new subscriptions.EmailSubscription(alarmEmail),
    );

    // Output the topic ARN for reference.
    new cdk.CfnOutput(this, 'AlarmTopicArn', {
      value: this.topic.topicArn,
      description: `SNS topic ARN for CloudWatch alarms (${environment})`,
    });
  }
}
