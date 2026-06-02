package resource

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
)

func GetSqsQueues(ctx context.Context, cfg aws.Config, tags map[string]string) ([]string, error) {
	arns, err := getARNs(ctx, cfg, []string{"sqs:queue"}, tags)
	if err != nil {
		return nil, err
	}

	queues := make([]string, 0, len(arns))
	for _, arnStr := range arns {
		queueName, err := extractQueueName(arnStr)
		if err != nil {
			return nil, err
		}
		queues = append(queues, queueName)
	}

	return queues, nil
}

// extractQueueName extracts the queue name from an SQS queue ARN that has
// this format: arn:aws:sqs:region:account:queue-name
func extractQueueName(arnStr string) (string, error) {
	parsedArn, err := arn.Parse(arnStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse arn: %w", err)
	}

	if parsedArn.Service != "sqs" {
		return "", fmt.Errorf("not sqs queue arn: %s", arnStr)
	}

	// For SQS, the Resource is simply the queue name
	queueName := strings.TrimSpace(parsedArn.Resource)
	if queueName == "" {
		return "", fmt.Errorf("empty queue name in arn: %s", arnStr)
	}

	return queueName, nil
}
