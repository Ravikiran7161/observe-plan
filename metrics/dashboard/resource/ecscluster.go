package resource

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
)

func GetECSClusters(ctx context.Context, cfg aws.Config, tags map[string]string) ([]string, error) {
	arns, err := getARNs(ctx, cfg, []string{"ecs:cluster"}, tags)
	if err != nil {
		return nil, err
	}

	clusters := make([]string, 0, len(arns))
	for _, arnStr := range arns {
		cluster, err := extractNameFromARN(arnStr, "ecs", "cluster")
		if err != nil {
			return nil, err
		}
		clusters = append(clusters, cluster)
	}

	return clusters, nil
}
