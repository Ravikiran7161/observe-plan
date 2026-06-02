package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"

	"github.com/thermofisher/cmd-saas/observe-plane/metrics/dashboard/resource"
)

const NamePrefix = "cmd-saas-dynamic"

// Resources encapsulates all AWS resources to include in a dashboard.
type Resources struct {
	AmplifyApps      []resource.AmplifyApp
	CognitoUserPools []resource.CognitoUserPool
	FunctionNames    []string
	S3Buckets        []string
	DynamoDBTables   []string
	APIGateways      []string
	Queues           []string
	ECSClusters      []string
	EventBuses       []string
}

// Count returns the total number of resources across all supported resource types.
func (resources Resources) Count() int {
	return len(resources.AmplifyApps) +
		len(resources.CognitoUserPools) +
		len(resources.FunctionNames) +
		len(resources.S3Buckets) +
		len(resources.DynamoDBTables) +
		len(resources.APIGateways) +
		len(resources.Queues) +
		len(resources.ECSClusters) +
		len(resources.EventBuses)
}

// Put creates or updates a CloudWatch dashboard with metrics for the specified resources.
func Put(
	ctx context.Context,
	cfg aws.Config,
	name string,
	tags resource.Tags,
	resources Resources,
) error {
	body, err := body(cfg.Region, tags, resources)
	if err != nil {
		return fmt.Errorf("generating dashboard body: %w", err)
	}
	if body == "" {
		tagsJSON, err := json.Marshal(tags)
		if err != nil {
			return fmt.Errorf("marshaling tags: %w", err)
		}
		return fmt.Errorf("generated empty dashboard body for the specified tags: %s", tagsJSON)
	}

	if err := put(ctx, cfg, name, body); err != nil {
		return fmt.Errorf("putting dashboard %q: %w", name, err)
	}
	return nil
}

// Delete removes one or more CloudWatch dashboards by name.
func Delete(ctx context.Context, cfg aws.Config, names []string) error {
	if len(names) == 0 {
		return nil
	}

	client := cloudwatch.NewFromConfig(cfg)
	_, err := client.DeleteDashboards(ctx, &cloudwatch.DeleteDashboardsInput{
		DashboardNames: names,
	})
	if err != nil {
		return fmt.Errorf("deleting dashboards %v: %w", names, err)
	}
	return nil
}

// ListNamesByPrefix returns CloudWatch dashboard names that match a prefix.
func ListNamesByPrefix(ctx context.Context, cfg aws.Config, prefix string) ([]string, error) {
	client := cloudwatch.NewFromConfig(cfg)

	names := make([]string, 0)
	var nextToken *string

	for {
		output, err := client.ListDashboards(ctx, &cloudwatch.ListDashboardsInput{
			DashboardNamePrefix: aws.String(prefix),
			NextToken:           nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("listing dashboards by prefix %q: %w", prefix, err)
		}

		for _, entry := range output.DashboardEntries {
			if entry.DashboardName == nil {
				continue
			}
			names = append(names, *entry.DashboardName)
		}

		if output.NextToken == nil || *output.NextToken == "" {
			break
		}
		nextToken = output.NextToken
	}

	return names, nil
}

// DeleteByPrefix removes all CloudWatch dashboards whose names match the prefix.
func DeleteByPrefix(ctx context.Context, cfg aws.Config, prefix string) error {
	names, err := ListNamesByPrefix(ctx, cfg, prefix)
	if err != nil {
		return err
	}

	if err := Delete(ctx, cfg, names); err != nil {
		return err
	}

	return nil
}

func put(ctx context.Context, cfg aws.Config, name, body string) error {
	client := cloudwatch.NewFromConfig(cfg)
	_, err := client.PutDashboard(ctx, &cloudwatch.PutDashboardInput{
		DashboardName: aws.String(name),
		DashboardBody: aws.String(body),
	})
	if err != nil {
		return fmt.Errorf("CloudWatch API call failed: %w", err)
	}
	return nil
}

// header generates the markdown header for the dashboard with tag information.
func header(tags map[string]string) string {
	if len(tags) == 0 {
		return "# Dynamically generated dashboard"
	}

	// Sort keys for consistent output
	keys := make([]string, 0, len(tags))
	for key := range tags {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	// Build tag list
	tagStrings := make([]string, len(keys))
	for i, key := range keys {
		tagStrings[i] = fmt.Sprintf("**%s**=%s", key, tags[key])
	}
	tagsList := strings.Join(tagStrings, ", ")

	return "# Dynamically generated dashboard\nfor resources with these tags: " + tagsList
}

func body(region string, tags map[string]string, res Resources) (string, error) {
	widgets := []Widget{NewTextWidget(header(tags), 24, 2)}

	widgets = addAmplifyWidgets(widgets, region, res.AmplifyApps)
	widgets = addAPIGatewayWidgets(widgets, region, res.APIGateways)
	widgets = addCognitoWidgets(widgets, region, res.CognitoUserPools)
	widgets = addSQSWidgets(widgets, region, res.Queues)
	widgets = addLambdaWidgets(widgets, region, res.FunctionNames)
	widgets = addDynamoDBWidgets(widgets, region, res.DynamoDBTables)
	widgets = addS3Widgets(widgets, region, res.S3Buckets)
	widgets = addECSClusterWidgets(widgets, region, res.ECSClusters)
	widgets = addEventBusWidgets(widgets, region, res.EventBuses)

	dashboard := struct {
		Widgets []Widget `json:"widgets"`
	}{Widgets: widgets}

	b, err := json.Marshal(dashboard)
	if err != nil {
		return "", fmt.Errorf("marshaling dashboard JSON: %w", err)
	}

	return string(b), nil
}
