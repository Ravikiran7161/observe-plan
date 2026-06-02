// Package resource — tagged.go
//
// This file adds "WithTags" variants of the existing discovery functions.
// They return structured results that include the resource-level tags,
// enabling callers to group resources by plane/domain/tenant-code.
//
// IMPORTANT: None of the existing functions are modified.
// All new types and functions are purely additive.

package resource

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	apigwsdk "github.com/aws/aws-sdk-go-v2/service/apigateway"
	apigwtypes "github.com/aws/aws-sdk-go-v2/service/apigateway/types"
	"github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi"
	"github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi/types"
)

// --- Generic tagged result type ---

// TaggedResource is a named AWS resource with its tag map.
type TaggedResource struct {
	Name string
	Tags map[string]string
}

// TaggedRestApiGateway is an API Gateway REST API with stage and tag map.
type TaggedRestApiGateway struct {
	Name  string
	Stage string
	Tags  map[string]string
}

// TaggedAmplifyApp is an Amplify app with its tag map.
type TaggedAmplifyApp struct {
	ID   string
	Name string
	Tags map[string]string
}

// TaggedCognitoUserPool is a Cognito user pool with client IDs and tag map.
type TaggedCognitoUserPool struct {
	ID        string
	ClientIDs []string
	Tags      map[string]string
}

// --- Internal helper: fetch ARNs with their tags ---

// arnWithTags bundles a resource ARN with the tags AWS returned for it.
type arnWithTags struct {
	ARN  string
	Tags map[string]string
}

func getARNsWithTags(
	ctx context.Context,
	cfg aws.Config,
	resourceTypeFilters []string,
	filterTags map[string]string,
) ([]arnWithTags, error) {
	client := resourcegroupstaggingapi.NewFromConfig(cfg)

	awsTagFilters := make([]types.TagFilter, 0, len(filterTags))
	for k, v := range filterTags {
		awsTagFilters = append(awsTagFilters, types.TagFilter{
			Key:    aws.String(k),
			Values: []string{v},
		})
	}

	var results []arnWithTags
	var nextToken *string

	for {
		out, err := client.GetResources(ctx, &resourcegroupstaggingapi.GetResourcesInput{
			ResourceTypeFilters: resourceTypeFilters,
			TagFilters:          awsTagFilters,
			PaginationToken:     nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get resources with tags: %w", err)
		}
		for _, r := range out.ResourceTagMappingList {
			if r.ResourceARN == nil {
				continue
			}
			tags := make(map[string]string, len(r.Tags))
			for _, t := range r.Tags {
				if t.Key != nil && t.Value != nil {
					tags[*t.Key] = *t.Value
				}
			}
			results = append(results, arnWithTags{ARN: *r.ResourceARN, Tags: tags})
		}
		if out.PaginationToken == nil || *out.PaginationToken == "" {
			break
		}
		nextToken = out.PaginationToken
	}
	return results, nil
}

// --- WithTags variants ---

// GetLambdaFunctionsWithTags returns Lambda functions matching the filter tags,
// each carrying its own resource-level tag map.
func GetLambdaFunctionsWithTags(
	ctx context.Context,
	cfg aws.Config,
	filterTags map[string]string,
) ([]TaggedResource, error) {
	items, err := getARNsWithTags(ctx, cfg, []string{"lambda:function"}, filterTags)
	if err != nil {
		return nil, err
	}
	results := make([]TaggedResource, 0, len(items))
	for _, item := range items {
		name, err := extractFunctionName(item.ARN)
		if err != nil {
			return nil, err
		}
		results = append(results, TaggedResource{Name: name, Tags: item.Tags})
	}
	return results, nil
}

// GetApiGatewaysWithDetailsAndTags returns REST API Gateways matching the filter tags,
// including the deployed stage name and resource-level tag map.
func GetApiGatewaysWithDetailsAndTags(
	ctx context.Context,
	cfg aws.Config,
	filterTags map[string]string,
) ([]TaggedRestApiGateway, error) {
	items, err := getARNsWithTags(ctx, cfg, []string{"apigateway:restapis"}, filterTags)
	if err != nil {
		return nil, err
	}

	client := apigwsdk.NewFromConfig(cfg)

	type apiStageKey struct{ id, stage string }
	seen := make(map[apiStageKey]bool)

	var results []TaggedRestApiGateway

	for _, item := range items {
		apiID, stage, err := extractApiIdAndStage(item.ARN)
		if err != nil {
			return nil, err
		}
		key := apiStageKey{id: apiID, stage: stage}
		if seen[key] {
			continue
		}
		seen[key] = true

		out, err := client.GetRestApi(ctx, &apigwsdk.GetRestApiInput{RestApiId: aws.String(apiID)})
		if err != nil {
			var notFound *apigwtypes.NotFoundException
			if errors.As(err, &notFound) {
				continue
			}
			return nil, fmt.Errorf("failed to get REST API details for %s: %w", apiID, err)
		}
		if out.Name == nil {
			continue
		}

		if stage != "" {
			stageKey := apiStageKey{id: apiID, stage: stage}
			if !seen[stageKey] {
				seen[stageKey] = true
				results = append(results, TaggedRestApiGateway{
					Name: *out.Name, Stage: stage, Tags: item.Tags,
				})
			}
			continue
		}

		stages, err := getDeployedStages(ctx, client, apiID)
		if err != nil {
			return nil, err
		}
		for _, s := range stages {
			stageKey := apiStageKey{id: apiID, stage: s}
			if seen[stageKey] {
				continue
			}
			seen[stageKey] = true
			results = append(results, TaggedRestApiGateway{
				Name: *out.Name, Stage: s, Tags: item.Tags,
			})
		}
	}
	return results, nil
}

// GetDynamoDBTablesWithTags returns DynamoDB tables matching the filter tags,
// each carrying its own resource-level tag map.
func GetDynamoDBTablesWithTags(
	ctx context.Context,
	cfg aws.Config,
	filterTags map[string]string,
) ([]TaggedResource, error) {
	items, err := getARNsWithTags(ctx, cfg, []string{"dynamodb:table"}, filterTags)
	if err != nil {
		return nil, err
	}
	results := make([]TaggedResource, 0, len(items))
	for _, item := range items {
		name, err := extractNameFromARN(item.ARN, "dynamodb", "table")
		if err != nil {
			return nil, err
		}
		results = append(results, TaggedResource{Name: name, Tags: item.Tags})
	}
	return results, nil
}

// GetSqsQueuesWithTags returns SQS queues matching the filter tags,
// each carrying its own resource-level tag map.
func GetSqsQueuesWithTags(
	ctx context.Context,
	cfg aws.Config,
	filterTags map[string]string,
) ([]TaggedResource, error) {
	items, err := getARNsWithTags(ctx, cfg, []string{"sqs:queue"}, filterTags)
	if err != nil {
		return nil, err
	}
	results := make([]TaggedResource, 0, len(items))
	for _, item := range items {
		name, err := extractQueueName(item.ARN)
		if err != nil {
			return nil, err
		}
		results = append(results, TaggedResource{Name: name, Tags: item.Tags})
	}
	return results, nil
}

// GetECSClustersWithTags returns ECS clusters matching the filter tags,
// each carrying its own resource-level tag map.
func GetECSClustersWithTags(
	ctx context.Context,
	cfg aws.Config,
	filterTags map[string]string,
) ([]TaggedResource, error) {
	items, err := getARNsWithTags(ctx, cfg, []string{"ecs:cluster"}, filterTags)
	if err != nil {
		return nil, err
	}
	results := make([]TaggedResource, 0, len(items))
	for _, item := range items {
		name, err := extractNameFromARN(item.ARN, "ecs", "cluster")
		if err != nil {
			return nil, err
		}
		results = append(results, TaggedResource{Name: name, Tags: item.Tags})
	}
	return results, nil
}

// GetEventBusesWithTags returns EventBridge buses matching the filter tags,
// each carrying its own resource-level tag map.
func GetEventBusesWithTags(
	ctx context.Context,
	cfg aws.Config,
	filterTags map[string]string,
) ([]TaggedResource, error) {
	items, err := getARNsWithTags(ctx, cfg, []string{"events:event-bus"}, filterTags)
	if err != nil {
		return nil, err
	}
	results := make([]TaggedResource, 0, len(items))
	for _, item := range items {
		name, err := extractNameFromARN(item.ARN, "events", "event-bus")
		if err != nil {
			return nil, err
		}
		results = append(results, TaggedResource{Name: name, Tags: item.Tags})
	}
	return results, nil
}

// GetS3BucketsWithTags returns S3 buckets matching the filter tags,
// each carrying its own resource-level tag map.
func GetS3BucketsWithTags(
	ctx context.Context,
	cfg aws.Config,
	filterTags map[string]string,
) ([]TaggedResource, error) {
	items, err := getARNsWithTags(ctx, cfg, []string{"s3"}, filterTags)
	if err != nil {
		return nil, err
	}
	results := make([]TaggedResource, 0, len(items))
	for _, item := range items {
		name, err := extractBucketName(item.ARN)
		if err != nil {
			return nil, err
		}
		results = append(results, TaggedResource{Name: name, Tags: item.Tags})
	}
	return results, nil
}

// GetAmplifyAppsWithTags returns Amplify apps matching the filter tags,
// each carrying its own resource-level tag map.
// Note: Amplify uses a ListApps + tag-check pattern (same as existing GetAmplifyApps).
func GetAmplifyAppsWithTags(
	ctx context.Context,
	cfg aws.Config,
	filterTags map[string]string,
) ([]TaggedAmplifyApp, error) {
	// Reuse existing GetAmplifyApps which already fetches tags internally.
	// We re-fetch tags via the tagging API to get the full tag map.
	items, err := getARNsWithTags(ctx, cfg, []string{"amplify:apps"}, filterTags)
	if err != nil {
		// Amplify may not be supported by the tagging API in all regions;
		// fall back to the existing list-based approach without tags.
		existing, fallbackErr := GetAmplifyApps(ctx, cfg, filterTags)
		if fallbackErr != nil {
			return nil, fmt.Errorf("discovering Amplify apps (fallback): %w", fallbackErr)
		}
		results := make([]TaggedAmplifyApp, 0, len(existing))
		for _, a := range existing {
			results = append(results, TaggedAmplifyApp{ID: a.ID, Name: a.Name, Tags: filterTags})
		}
		return results, nil
	}

	results := make([]TaggedAmplifyApp, 0, len(items))
	for _, item := range items {
		// Extract app ID from ARN: arn:aws:amplify:region:account:apps/app-id
		parsedArn, err := arn.Parse(item.ARN)
		if err != nil {
			continue
		}
		parts := strings.SplitN(parsedArn.Resource, "/", 2)
		if len(parts) != 2 {
			continue
		}
		appID := parts[1]
		name := item.Tags["name"]
		if name == "" {
			name = appID
		}
		results = append(results, TaggedAmplifyApp{ID: appID, Name: name, Tags: item.Tags})
	}
	return results, nil
}

// GetCognitoUserPoolsWithTags returns Cognito user pools matching the filter tags,
// each carrying its client IDs and its own resource-level tag map.
// Delegates to existing GetCognitoUserPools for the pool+client discovery,
// then fetches tags via the tagging API.
func GetCognitoUserPoolsWithTags(
	ctx context.Context,
	cfg aws.Config,
	filterTags map[string]string,
) ([]TaggedCognitoUserPool, error) {
	pools, err := GetCognitoUserPools(ctx, cfg, filterTags)
	if err != nil {
		return nil, err
	}

	accountID, err := getAccountID(ctx, cfg)
	if err != nil {
		return nil, err
	}

	results := make([]TaggedCognitoUserPool, 0, len(pools))
	for _, p := range pools {
		poolTags, err := getUserPoolTagsWithAccountID(ctx, cfg, p.ID, accountID)
		if err != nil {
			poolTags = filterTags // fall back to filter tags on error
		}
		if poolTags == nil {
			poolTags = filterTags
		}
		results = append(results, TaggedCognitoUserPool{
			ID:        p.ID,
			ClientIDs: p.ClientIDs,
			Tags:      poolTags,
		})
	}
	return results, nil
}
