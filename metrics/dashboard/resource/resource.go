package resource

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi"
	"github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi/types"
)

func getARNs(
	ctx context.Context,
	cfg aws.Config,
	resourceTypeFilters []string,
	tags map[string]string,
) ([]string, error) {
	client := resourcegroupstaggingapi.NewFromConfig(cfg)

	var ARNs []string

	var nextToken *string
	awsTagFilters := make([]types.TagFilter, 0, len(tags))
	keys := make([]string, 0, len(tags))
	for key := range tags {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		value := tags[key]
		awsTagFilters = append(awsTagFilters, types.TagFilter{
			Key:    aws.String(key),
			Values: []string{value},
		})
	}

	for {
		input := &resourcegroupstaggingapi.GetResourcesInput{
			ResourceTypeFilters: resourceTypeFilters,
			TagFilters:          awsTagFilters,
			PaginationToken:     nextToken,
		}

		output, err := client.GetResources(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("failed to get resources: %w", err)
		}

		// Extract function ARNs from response
		for _, resource := range output.ResourceTagMappingList {
			if resource.ResourceARN != nil {
				ARNs = append(ARNs, *resource.ResourceARN)
			}
		}

		// Check if there are more pages
		if output.PaginationToken == nil || *output.PaginationToken == "" {
			break
		}
		nextToken = output.PaginationToken
	}

	return ARNs, nil
}

func matchesAllTags(tags map[string]string, want map[string]string) bool {
	if len(want) == 0 {
		return true
	}
	for key, value := range want {
		val, exists := tags[key]
		if !exists || val != value {
			return false
		}
	}
	return true
}

// extractNameFromARN extracts a resource name from an ARN given a service name
// and resource type prefix.
func extractNameFromARN(arnStr string, service string, resourcePrefix string) (string, error) {
	parsedArn, err := arn.Parse(arnStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse arn: %w", err)
	}

	if !strings.HasSuffix(resourcePrefix, "/") {
		resourcePrefix += "/"
	}

	if parsedArn.Service != service {
		return "", fmt.Errorf("not %s arn: %s", service, arnStr)
	}

	if !strings.HasPrefix(parsedArn.Resource, resourcePrefix) {
		return "", fmt.Errorf("resource does not match expected prefix in arn: %s", arnStr)
	}

	name := strings.TrimPrefix(parsedArn.Resource, resourcePrefix)
	if name == "" {
		return "", fmt.Errorf("empty resource name in arn: %s", arnStr)
	}

	return name, nil
}
