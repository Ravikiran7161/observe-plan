package resource

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/service/apigateway"
	"github.com/aws/aws-sdk-go-v2/service/apigateway/types"
)

type RestApiGateway struct {
	Name  string
	Stage string
}

// GetApiGatewaysWithDetails returns REST API Gateways matching the given tags,
// including the stage name which is needed for CloudWatch alarm dimensions
// Each API may appear multiple times if it has multiple stages, all stages are returned
func GetApiGatewaysWithDetails(ctx context.Context, cfg aws.Config, tags map[string]string) ([]RestApiGateway, error) {
	arns, err := getARNs(ctx, cfg, []string{"apigateway:restapis"}, tags)
	if err != nil {
		return nil, err
	}

	client := apigateway.NewFromConfig(cfg)
	var results []RestApiGateway
	type apiStageKey struct{ id, stage string }
	seen := make(map[apiStageKey]bool)

	for _, arnStr := range arns {
		apiID, stage, err := extractApiIdAndStage(arnStr)
		if err != nil {
			return nil, err
		}

		key := apiStageKey{id: apiID, stage: stage}
		if seen[key] {
			continue
		}
		seen[key] = true

		output, err := client.GetRestApi(ctx, &apigateway.GetRestApiInput{
			RestApiId: aws.String(apiID),
		})
		if err != nil {
			var notFoundErr *types.NotFoundException
			if errors.As(err, &notFoundErr) {
				continue
			}
			return nil, fmt.Errorf("failed to get REST API details for %s: %w", apiID, err)
		}

		if output.Name == nil {
			continue
		}

		// If the ARN included a stage, use it directly
		// Otherwise, list all deployed stages for this API
		if stage != "" {
			results = append(results, RestApiGateway{Name: *output.Name, Stage: stage})
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
			results = append(results, RestApiGateway{Name: *output.Name, Stage: s})
		}
	}

	return results, nil
}

// getDeployedStages returns all stage names for a given REST API
func getDeployedStages(ctx context.Context, client *apigateway.Client, apiID string) ([]string, error) {
	out, err := client.GetStages(ctx, &apigateway.GetStagesInput{
		RestApiId: aws.String(apiID),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list stages for API %s: %w", apiID, err)
	}
	stages := make([]string, 0, len(out.Item))
	for _, s := range out.Item {
		if s.StageName != nil {
			stages = append(stages, *s.StageName)
		}
	}
	return stages, nil
}

func GetApiGateways(ctx context.Context, cfg aws.Config, tags map[string]string) ([]string, error) {
	arns, err := getARNs(ctx, cfg, []string{"apigateway:restapis"}, tags)
	if err != nil {
		return nil, err
	}

	client := apigateway.NewFromConfig(cfg)
	var apiNames []string
	seenAPIIds := make(map[string]bool) // Deduplicate API IDs (stages might create duplicates)

	for _, arn := range arns {
		apiID, err := extractApiId(arn)
		if err != nil {
			return nil, err
		}

		// Skip if we've already processed this API ID
		if seenAPIIds[apiID] {
			continue
		}
		seenAPIIds[apiID] = true

		// Get the REST API details to retrieve the name
		output, err := client.GetRestApi(ctx, &apigateway.GetRestApiInput{
			RestApiId: aws.String(apiID),
		})
		if err != nil {
			// Skip APIs that no longer exist (404 errors) but still have tags
			// This can happen when resources are deleted but tags haven't been cleaned up yet
			var notFoundErr *types.NotFoundException
			if errors.As(err, &notFoundErr) {
				continue
			}
			return nil, fmt.Errorf("failed to get REST API details for %s: %w", apiID, err)
		}

		if output.Name != nil {
			apiNames = append(apiNames, *output.Name)
		}
	}

	return apiNames, nil
}

// extractApiId extracts the API ID from an API Gateway ARN
// API Gateway ARN format: arn:aws:apigateway:region::/restapis/api-id
// or arn:aws:apigateway:region::/restapis/api-id/stages/stage-name
func extractApiId(arnStr string) (string, error) {
	apiID, _, err := extractApiIdAndStage(arnStr)
	return apiID, err
}

// extractApiIdAndStage extracts the API ID and, if present, the stage name from
// an API Gateway ARN
// Returns (apiID, stage, error). stage is empty when the ARN has no stage
func extractApiIdAndStage(arnStr string) (string, string, error) {
	parsedArn, err := arn.Parse(arnStr)
	if err != nil {
		return "", "", fmt.Errorf("failed to parse ARN: %w", err)
	}

	const prefix = "/restapis/"
	if !strings.HasPrefix(parsedArn.Resource, prefix) {
		return "", "", fmt.Errorf("invalid API Gateway ARN format: %s", arnStr)
	}

	resourcePath := strings.TrimPrefix(parsedArn.Resource, prefix)
	// Format: api-id  or  api-id/stages/stage-name
	parts := strings.Split(resourcePath, "/")
	apiID := parts[0]
	if apiID == "" {
		return "", "", fmt.Errorf("empty API ID in ARN: %s", arnStr)
	}

	// parts: [api-id, "stages", stage-name]
	var stage string
	if len(parts) >= 3 && parts[1] == "stages" {
		stage = parts[2]
	}

	return apiID, stage, nil
}
