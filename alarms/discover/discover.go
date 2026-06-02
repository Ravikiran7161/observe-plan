// Command discover queries AWS for all resources matching the tags defined in a
// config file, groups them by plane/domain/tenant-code, and writes one inventory
// file per scope. The CDK launcher reads these files to create one alarm stack
// per scope.
//
// Usage:
//
//	go run ./discover --config <path/to/config.json>
//
// Output:
//
//	generated/multi/manifest.json    — list of scope names
//	generated/multi/control.json     — plane=control resources
//	generated/multi/app-drive.json   — plane=app, domain=drive resources
//	generated/multi/tenant-acme.json — tenant-code=acme resources

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"

	"github.com/thermofisher/cmd-saas/observe-plane/metrics/dashboard/resource"
)

// --- config schema ---

type discoverConfig struct {
	Tags map[string]string `json:"tags"`
}

// --- resource entry types ---

type lambdaEntry struct {
	FunctionName string `json:"functionName"`
}

type apiGatewayEntry struct {
	Kind    string `json:"kind"`
	APIName string `json:"apiName"`
	Stage   string `json:"stage"`
}

type dynamoDBEntry struct {
	TableName string `json:"tableName"`
}

type sqsEntry struct {
	QueueName string `json:"queueName"`
}

type ecsClusterEntry struct {
	ClusterName string `json:"clusterName"`
}

type eventBusEntry struct {
	EventBusName string `json:"eventBusName"`
}

type s3BucketEntry struct {
	BucketName string `json:"bucketName"`
}

type amplifyAppEntry struct {
	AppID string `json:"appId"`
	Name  string `json:"name"`
}

type cognitoUserPoolEntry struct {
	UserPoolID string   `json:"userPoolId"`
	ClientIDs  []string `json:"clientIds"`
}

// --- per-scope inventory schema ---

type scopeInventory struct {
	Scope       string            `json:"scope"`
	Tags        map[string]string `json:"tags"`
	GeneratedAt string            `json:"generatedAt"`
	Resources   struct {
		Lambdas          []lambdaEntry          `json:"lambdas"`
		APIGateways      []apiGatewayEntry      `json:"apiGateways"`
		DynamoDBTables   []dynamoDBEntry        `json:"dynamoDBTables"`
		SQSQueues        []sqsEntry             `json:"sqsQueues"`
		ECSClusters      []ecsClusterEntry      `json:"ecsClusters"`
		EventBuses       []eventBusEntry        `json:"eventBuses"`
		S3Buckets        []s3BucketEntry        `json:"s3Buckets"`
		AmplifyApps      []amplifyAppEntry      `json:"amplifyApps"`
		CognitoUserPools []cognitoUserPoolEntry `json:"cognitoUserPools"`
	} `json:"resources"`
}

// --- scope key ---

type scopeKey struct {
	scopeType string // "control", "app", or "tenant"
	qualifier string // domain for app, tenant-code for tenant, empty for control
}

func (k scopeKey) name() string {
	switch k.scopeType {
	case "tenant":
		return "tenant-" + k.qualifier
	case "app":
		if k.qualifier != "" {
			return "app-" + k.qualifier
		}
		return "app"
	default:
		return "control"
	}
}

func scopeTagsFor(key scopeKey, environment string) map[string]string {
	t := map[string]string{"environment": environment}
	switch key.scopeType {
	case "control":
		t["plane"] = "control"
	case "app":
		t["plane"] = "app"
		if key.qualifier != "" {
			t["domain"] = key.qualifier
		}
	case "tenant":
		t["tenant-code"] = key.qualifier
	}
	return t
}

// tagToScope maps a resource's tags to the appropriate stack scope.
func tagToScope(resourceTags map[string]string) (scopeKey, bool) {
	if tc := resourceTags["tenant-code"]; tc != "" {
		return scopeKey{scopeType: "tenant", qualifier: strings.ToLower(tc)}, true
	}
	switch strings.ToLower(resourceTags["plane"]) {
	case "control":
		return scopeKey{scopeType: "control"}, true
	case "app":
		domain := strings.ToLower(resourceTags["domain"])
		return scopeKey{scopeType: "app", qualifier: domain}, true
	}
	return scopeKey{}, false
}

// --- main ---

func main() {
	configPath := flag.String("config", "", "Path to the tag config JSON file (required)")
	flag.Parse()

	if *configPath == "" {
		fmt.Fprintln(os.Stderr, "Usage: discover --config <config.json>")
		os.Exit(1)
	}

	if err := run(*configPath); err != nil {
		slog.Error("discover failed", "error", err)
		os.Exit(1)
	}
}

func run(configPath string) error {
	cfg, err := loadConfig(configPath)
	if err != nil {
		return fmt.Errorf("loading config %q: %w", configPath, err)
	}
	if len(cfg.Tags) == 0 {
		return fmt.Errorf("config file %q must contain at least one tag", configPath)
	}
	environment := cfg.Tags["environment"]
	if environment == "" {
		return fmt.Errorf("config file %q must contain an 'environment' tag", configPath)
	}

	ctx := context.Background()
	awsCfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("loading AWS config: %w", err)
	}

	slog.Info("discovering resources", "tags", cfg.Tags, "region", awsCfg.Region)

	scopes := make(map[scopeKey]*scopeInventory)
	getOrCreate := func(key scopeKey) *scopeInventory {
		if _, ok := scopes[key]; !ok {
			scopes[key] = &scopeInventory{
				Scope:       key.name(),
				Tags:        scopeTagsFor(key, environment),
				GeneratedAt: time.Now().UTC().Format(time.RFC3339),
			}
		}
		return scopes[key]
	}

	// Lambda
	lambdas, err := resource.GetLambdaFunctionsWithTags(ctx, awsCfg, cfg.Tags)
	if err != nil {
		return fmt.Errorf("discovering Lambda functions: %w", err)
	}
	slog.Info("discovered Lambda functions", "count", len(lambdas))
	for _, fn := range lambdas {
		key, ok := tagToScope(fn.Tags)
		if !ok {
			continue
		}
		getOrCreate(key).Resources.Lambdas = append(getOrCreate(key).Resources.Lambdas, lambdaEntry{FunctionName: fn.Name})
	}

	// API Gateway
	apis, err := resource.GetApiGatewaysWithDetailsAndTags(ctx, awsCfg, cfg.Tags)
	if err != nil {
		return fmt.Errorf("discovering API Gateways: %w", err)
	}
	slog.Info("discovered API Gateways", "count", len(apis))
	for _, api := range apis {
		key, ok := tagToScope(api.Tags)
		if !ok {
			continue
		}
		getOrCreate(key).Resources.APIGateways = append(getOrCreate(key).Resources.APIGateways, apiGatewayEntry{
			Kind: "rest", APIName: api.Name, Stage: api.Stage,
		})
	}

	// DynamoDB
	tables, err := resource.GetDynamoDBTablesWithTags(ctx, awsCfg, cfg.Tags)
	if err != nil {
		return fmt.Errorf("discovering DynamoDB tables: %w", err)
	}
	slog.Info("discovered DynamoDB tables", "count", len(tables))
	for _, t := range tables {
		key, ok := tagToScope(t.Tags)
		if !ok {
			continue
		}
		getOrCreate(key).Resources.DynamoDBTables = append(getOrCreate(key).Resources.DynamoDBTables, dynamoDBEntry{TableName: t.Name})
	}

	// SQS
	queues, err := resource.GetSqsQueuesWithTags(ctx, awsCfg, cfg.Tags)
	if err != nil {
		return fmt.Errorf("discovering SQS queues: %w", err)
	}
	slog.Info("discovered SQS queues", "count", len(queues))
	for _, q := range queues {
		key, ok := tagToScope(q.Tags)
		if !ok {
			continue
		}
		getOrCreate(key).Resources.SQSQueues = append(getOrCreate(key).Resources.SQSQueues, sqsEntry{QueueName: q.Name})
	}

	// ECS
	clusters, err := resource.GetECSClustersWithTags(ctx, awsCfg, cfg.Tags)
	if err != nil {
		return fmt.Errorf("discovering ECS clusters: %w", err)
	}
	slog.Info("discovered ECS clusters", "count", len(clusters))
	for _, c := range clusters {
		key, ok := tagToScope(c.Tags)
		if !ok {
			continue
		}
		getOrCreate(key).Resources.ECSClusters = append(getOrCreate(key).Resources.ECSClusters, ecsClusterEntry{ClusterName: c.Name})
	}

	// EventBridge
	buses, err := resource.GetEventBusesWithTags(ctx, awsCfg, cfg.Tags)
	if err != nil {
		return fmt.Errorf("discovering EventBridge buses: %w", err)
	}
	slog.Info("discovered EventBridge buses", "count", len(buses))
	for _, eb := range buses {
		key, ok := tagToScope(eb.Tags)
		if !ok {
			continue
		}
		getOrCreate(key).Resources.EventBuses = append(getOrCreate(key).Resources.EventBuses, eventBusEntry{EventBusName: eb.Name})
	}

	// S3
	buckets, err := resource.GetS3BucketsWithTags(ctx, awsCfg, cfg.Tags)
	if err != nil {
		return fmt.Errorf("discovering S3 buckets: %w", err)
	}
	slog.Info("discovered S3 buckets", "count", len(buckets))
	for _, b := range buckets {
		key, ok := tagToScope(b.Tags)
		if !ok {
			continue
		}
		getOrCreate(key).Resources.S3Buckets = append(getOrCreate(key).Resources.S3Buckets, s3BucketEntry{BucketName: b.Name})
	}

	// Amplify
	amplifyApps, err := resource.GetAmplifyAppsWithTags(ctx, awsCfg, cfg.Tags)
	if err != nil {
		return fmt.Errorf("discovering Amplify apps: %w", err)
	}
	slog.Info("discovered Amplify apps", "count", len(amplifyApps))
	for _, a := range amplifyApps {
		key, ok := tagToScope(a.Tags)
		if !ok {
			continue
		}
		getOrCreate(key).Resources.AmplifyApps = append(getOrCreate(key).Resources.AmplifyApps, amplifyAppEntry{AppID: a.ID, Name: a.Name})
	}

	// Cognito
	pools, err := resource.GetCognitoUserPoolsWithTags(ctx, awsCfg, cfg.Tags)
	if err != nil {
		return fmt.Errorf("discovering Cognito user pools: %w", err)
	}
	slog.Info("discovered Cognito user pools", "count", len(pools))
	for _, p := range pools {
		key, ok := tagToScope(p.Tags)
		if !ok {
			continue
		}
		getOrCreate(key).Resources.CognitoUserPools = append(getOrCreate(key).Resources.CognitoUserPools, cognitoUserPoolEntry{
			UserPoolID: p.ID, ClientIDs: p.ClientIDs,
		})
	}

	// Write output files
	outDir := filepath.Join(filepath.Dir(configPath), "..", "generated", "multi")
	if err := os.MkdirAll(filepath.Clean(outDir), 0o750); err != nil {
		return fmt.Errorf("creating output directory %q: %w", outDir, err)
	}

	manifest := make([]string, 0, len(scopes))
	for key, inv := range scopes {
		outPath := filepath.Join(filepath.Clean(outDir), key.name()+".json")
		if err := writeJSON(outPath, inv); err != nil {
			return fmt.Errorf("writing scope %q: %w", key.name(), err)
		}
		slog.Info("scope written", "scope", key.name(), "path", outPath)
		manifest = append(manifest, key.name())
	}

	manifestPath := filepath.Join(filepath.Clean(outDir), "manifest.json")
	if err := writeJSON(manifestPath, manifest); err != nil {
		return fmt.Errorf("writing manifest: %w", err)
	}
	slog.Info("discovery complete", "scopes", len(manifest), "manifest", manifestPath)
	return nil
}

// --- utils ---

func loadConfig(path string) (discoverConfig, error) {
	path = filepath.Clean(path)
	data, err := os.ReadFile(path)
	if err != nil {
		return discoverConfig{}, fmt.Errorf("reading file: %w", err)
	}
	var cfg discoverConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return discoverConfig{}, fmt.Errorf("parsing JSON: %w", err)
	}
	return cfg, nil
}

func writeJSON(path string, v any) error {
	path = filepath.Clean(path)
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling JSON: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("writing file: %w", err)
	}
	return nil
}
