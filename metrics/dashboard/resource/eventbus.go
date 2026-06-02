package resource

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
)

func GetEventBuses(ctx context.Context, cfg aws.Config, tags map[string]string) ([]string, error) {
	arns, err := getARNs(ctx, cfg, []string{"events:event-bus"}, tags)
	if err != nil {
		return nil, err
	}

	eventBuses := make([]string, 0, len(arns))
	for _, arnStr := range arns {
		eventBusName, err := extractNameFromARN(arnStr, "events", "event-bus")
		if err != nil {
			return nil, err
		}
		eventBuses = append(eventBuses, eventBusName)
	}

	return eventBuses, nil
}
