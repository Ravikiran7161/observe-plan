package main

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"

	"github.com/thermofisher/cmd-saas/libs/slogger"
	"github.com/thermofisher/cmd-saas/observe-plane/metrics/dashboard"
	"github.com/thermofisher/cmd-saas/observe-plane/metrics/dashboard/resource"
)

// Event is the input payload set on each EventBridge scheduled rule.
// Tags maps to resource tag filters (e.g. {"environment": "dev"}).
// Delete instructs the handler to remove the dashboard instead of creating it.
type Event struct {
	Tags            map[string]string `json:"tags"`
	Delete          bool              `json:"delete"`
	ResetAllManaged bool              `json:"resetAllManaged"`
}

func loadResources(ctx context.Context, cfg aws.Config, tags resource.Tags) (dashboard.Resources, error) {
	resources := dashboard.Resources{}

	apps, err := resource.GetAmplifyApps(ctx, cfg, tags)
	if err != nil {
		return resources, fmt.Errorf("getting amplify apps: %w", err)
	}
	resources.AmplifyApps = apps

	lambdas, err := resource.GetLambdaFunctions(ctx, cfg, tags)
	if err != nil {
		return resources, fmt.Errorf("getting lambda functions: %w", err)
	}
	resources.FunctionNames = lambdas

	pools, err := resource.GetCognitoUserPools(ctx, cfg, tags)
	if err != nil {
		return resources, fmt.Errorf("getting cognito user pools: %w", err)
	}
	resources.CognitoUserPools = pools

	buckets, err := resource.GetS3Buckets(ctx, cfg, tags)
	if err != nil {
		return resources, fmt.Errorf("getting s3 buckets: %w", err)
	}
	resources.S3Buckets = buckets

	tables, err := resource.GetDynamoDBTables(ctx, cfg, tags)
	if err != nil {
		return resources, fmt.Errorf("getting dynamodb tables: %w", err)
	}
	resources.DynamoDBTables = tables

	gateways, err := resource.GetApiGateways(ctx, cfg, tags)
	if err != nil {
		return resources, fmt.Errorf("getting api gateways: %w", err)
	}
	resources.APIGateways = gateways

	queues, err := resource.GetSqsQueues(ctx, cfg, tags)
	if err != nil {
		return resources, fmt.Errorf("getting sqs queues: %w", err)
	}
	resources.Queues = queues

	ecsClusters, err := resource.GetECSClusters(ctx, cfg, tags)
	if err != nil {
		return resources, fmt.Errorf("getting ecs clusters: %w", err)
	}
	resources.ECSClusters = ecsClusters

	eventBuses, err := resource.GetEventBuses(ctx, cfg, tags)
	if err != nil {
		return resources, fmt.Errorf("getting event buses: %w", err)
	}
	resources.EventBuses = eventBuses

	return resources, nil
}

func dashboardName(tags resource.Tags) string {
	tagSuffix := tags.String()
	if tagSuffix == "" {
		return dashboard.NamePrefix
	}
	return strings.Join([]string{dashboard.NamePrefix, tagSuffix}, "-")
}

func handle(ctx context.Context, event Event) error {
	tags := resource.Tags(event.Tags)
	name := dashboardName(tags)

	log := slogger.New().With(
		slogger.RequestIdAttr(ctx),
		slogger.LambdaEnvAttr(),
		slog.String("dashboardName", name),
		slog.Any("tags", map[string]string(tags)),
	)

	log.InfoContext(ctx, "dashboard handler invoked")

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.ErrorContext(ctx, "loading aws config failed", "error", err)
		return fmt.Errorf("loading AWS config: %w", err)
	}

	if event.ResetAllManaged {
		if delErr := dashboard.DeleteByPrefix(ctx, cfg, dashboard.NamePrefix); delErr != nil {
			log.ErrorContext(ctx, "resetting managed dashboards failed",
				"prefix", dashboard.NamePrefix, "error", delErr)
			return fmt.Errorf("resetting managed dashboards by prefix %q: %w", dashboard.NamePrefix, delErr)
		}
		return nil
	}

	if event.Delete {
		if delErr := dashboard.Delete(ctx, cfg, []string{name}); delErr != nil {
			log.ErrorContext(ctx, "deleting dashboard failed", "error", delErr)
			return fmt.Errorf("deleting dashboard %s: %w", name, delErr)
		}
		return nil
	}

	resources, err := loadResources(ctx, cfg, tags)
	if err != nil {
		log.ErrorContext(ctx, "loading dashboard resources failed", "error", err)
		return err
	}

	if resources.Count() == 0 {
		if err := dashboard.Delete(ctx, cfg, []string{name}); err != nil {
			log.ErrorContext(ctx, "deleting empty dashboard failed", "error", err)
			return fmt.Errorf("deleting empty dashboard %s: %w", name, err)
		}
		return nil
	}

	if err := dashboard.Put(ctx, cfg, name, tags, resources); err != nil {
		log.ErrorContext(ctx, "putting dashboard failed", "resourceCount", resources.Count(), "error", err)
		return fmt.Errorf("putting dashboard %s: %w", name, err)
	}

	return nil
}

func main() {
	lambda.Start(handle)
}
