package dashboard

import (
	"fmt"
	"strings"

	"github.com/thermofisher/cmd-saas/observe-plane/metrics/dashboard/resource"
)

// Color palette for dashboard widgets
const (
	colorDarkBlue   = "#1f77b4"
	colorLightBlue  = "#aec7e8"
	colorDarkGreen  = "#2ca02c"
	colorLightGreen = "#98df8a"
	colorDarkRed    = "#d62728"
	colorLightRed   = "#ff9896"
)

// CloudWatch statistic types
const (
	statSum = "Sum"
	statAvg = "Average"
	statMax = "Maximum"
	statMin = "Minimum"
	statP90 = "p90"
	statP75 = "p75"
)

// CloudWatch widget view types
const (
	viewTimeSeries = "timeSeries"
)

// CloudWatch metric property keys
const (
	keyColor  = "color"
	keyLabel  = "label"
	keyStat   = "stat"
	keyRegion = "region"
)

// colorPalette provides distinct colors for multi-resource widgets, one color per resource.
var colorPalette = []string{
	"#1f77b4", // blue
	"#2ca02c", // green
	"#ff7f0e", // orange
	"#9467bd", // purple
	"#8c564b", // brown
	"#e377c2", // pink
	"#17becf", // teal
	"#bcbd22", // yellow-green
	"#7f7f7f", // gray
	"#d62728", // red
}

func CognitoMetrics(region, userPool, userPoolClient, title string) WidgetProperties {
	return WidgetProperties{
		View:    viewTimeSeries,
		Stacked: false,
		Period:  Period,
		Title:   title,
		Region:  region,
		Metrics: [][]any{
			getCognitoMetric(userPool, userPoolClient, "SignUpSuccesses", colorDarkBlue, statSum),
			getCognitoMetric(userPool, userPoolClient, "SignInSuccesses", colorDarkGreen, statSum),
			getCognitoMetric(userPool, userPoolClient, "TokenRefreshSuccesses", colorLightGreen, statSum),
		},
		YAxis: &YAxis{
			Left: &YAxisSide{
				ShowUnits: false,
			},
		},
	}
}

func getCognitoMetric(userPool, userPoolClient, metric, color, stat string) []any {
	return []any{
		// [ "AWS/Cognito", "SignInSuccesses", "UserPool", "us-east-2_2VFWaPDZk", "UserPoolClient", "sl0bso78rm7577e45h7gpfrmp" ]
		"AWS/Cognito", metric, "UserPool", userPool, "UserPoolClient", userPoolClient,
		map[string]string{keyStat: stat, keyColor: color, keyLabel: metric},
	}
}

func CognitoSignUpMetrics(region string, userPools []resource.CognitoUserPool) WidgetProperties {
	metrics := getCognitoMetricsForPools(region, userPools, "SignUpSuccesses")
	return WidgetProperties{
		View:    viewTimeSeries,
		Stacked: false,
		Period:  Period,
		Title:   "Sign Up Successes",
		Region:  region,
		Metrics: metrics,
		YAxis: &YAxis{
			Left: &YAxisSide{
				ShowUnits: false,
			},
		},
	}
}

func CognitoSignInMetrics(region string, userPools []resource.CognitoUserPool) WidgetProperties {
	metrics := getCognitoMetricsForPools(region, userPools, "SignInSuccesses")
	return WidgetProperties{
		View:    viewTimeSeries,
		Stacked: false,
		Period:  Period,
		Title:   "Sign In Successes",
		Region:  region,
		Metrics: metrics,
		YAxis: &YAxis{
			Left: &YAxisSide{
				ShowUnits: false,
			},
		},
	}
}

func CognitoTokenRefreshMetrics(region string, userPools []resource.CognitoUserPool) WidgetProperties {
	metrics := getCognitoMetricsForPools(region, userPools, "TokenRefreshSuccesses")
	return WidgetProperties{
		View:    viewTimeSeries,
		Stacked: false,
		Period:  Period,
		Title:   "Token Refresh Successes",
		Region:  region,
		Metrics: metrics,
		YAxis: &YAxis{
			Left: &YAxisSide{
				ShowUnits: false,
			},
		},
	}
}

func getCognitoMetricsForPools(region string, userPools []resource.CognitoUserPool, metric string) [][]any {
	metrics := make([][]any, 0, len(userPools)*2)
	for i, userPool := range userPools {
		if len(userPool.ClientIDs) == 0 {
			continue
		}

		color := colorPalette[i%len(colorPalette)]
		clientMetricIDs := make([]string, 0, len(userPool.ClientIDs))
		for j, clientID := range userPool.ClientIDs {
			metricID := fmt.Sprintf("m%d_%d", i, j)
			clientMetricIDs = append(clientMetricIDs, metricID)
			metrics = append(metrics, getCognitoMetricHidden(region, userPool.ID, clientID, metric, metricID))
		}

		expressionID := fmt.Sprintf("e%d", i)
		metrics = append(metrics,
			getCognitoMetricAggregate(region, strings.Join(clientMetricIDs, " + "), userPool.ID, expressionID, color),
		)
	}

	return metrics
}

func getCognitoMetricHidden(region, userPool, userPoolClient, metric, id string) []any {
	return []any{
		"AWS/Cognito", metric, "UserPool", userPool, "UserPoolClient", userPoolClient,
		map[string]any{
			keyStat:   statSum,
			"id":      id,
			keyRegion: region,
			"visible": false,
		},
	}
}

func getCognitoMetricAggregate(region, expression, label, id, color string) []any {
	return []any{
		map[string]any{
			"expression": expression,
			keyLabel:     label,
			"id":         id,
			keyColor:     color,
			keyRegion:    region,
		},
	}
}

func LambdaMetrics(region, functionName, title string) WidgetProperties {
	return WidgetProperties{
		View:    viewTimeSeries,
		Stacked: false,
		Period:  Period,
		Title:   title,
		Region:  region,
		Metrics: [][]any{
			getLambdaMetric(functionName, "Invocations", colorDarkBlue, statSum),
			getLambdaMetric(functionName, "Errors", colorDarkRed, statSum),
			getSloggerExampleLambdaMetric(functionName, "ResponseErrors", colorLightRed, statSum),
		},
		YAxis: &YAxis{
			Left: &YAxisSide{
				ShowUnits: false,
			},
		},
	}
}

func getSloggerExampleLambdaMetric(functionName, metric, color, stat string) []any {
	return []any{
		"SloggerExample", metric, "FunctionName", functionName,
		map[string]string{keyStat: stat, keyColor: color, keyLabel: metric},
	}
}

func getLambdaMetricForFunction(functionName, functionLabel, metric, color, stat string) []any {
	return []any{
		"AWS/Lambda", metric, "FunctionName", functionName,
		map[string]string{keyStat: stat, keyColor: color, keyLabel: functionLabel},
	}
}

func LambdaInvocationsMetrics(region string, functions []string) WidgetProperties {
	metrics := make([][]any, 0, len(functions))
	for i, fn := range functions {
		color := colorPalette[i%len(colorPalette)]
		metrics = append(metrics, getLambdaMetricForFunction(fn, fn, "Invocations", color, statSum))
	}
	return WidgetProperties{
		View:    viewTimeSeries,
		Stacked: false,
		Period:  Period,
		Title:   "Invocations",
		Region:  region,
		Metrics: metrics,
		YAxis: &YAxis{
			Left: &YAxisSide{
				ShowUnits: false,
			},
		},
	}
}

func LambdaErrorsMetrics(region string, functions []string) WidgetProperties {
	metrics := make([][]any, 0, len(functions))
	for i, fn := range functions {
		color := colorPalette[i%len(colorPalette)]
		metrics = append(metrics, getLambdaMetricForFunction(fn, fn, "Errors", color, statSum))
	}
	return WidgetProperties{
		View:    viewTimeSeries,
		Stacked: false,
		Period:  Period,
		Title:   "Errors",
		Region:  region,
		Metrics: metrics,
		YAxis: &YAxis{
			Left: &YAxisSide{
				ShowUnits: false,
			},
		},
	}
}

func LambdaResponseErrorsMetrics(region string, functions []string) WidgetProperties {
	metrics := make([][]any, 0, len(functions))
	for i, fn := range functions {
		color := colorPalette[i%len(colorPalette)]
		metrics = append(metrics, []any{
			"SloggerExample", "ResponseErrors", "FunctionName", fn,
			map[string]string{keyStat: statSum, keyColor: color, keyLabel: fn},
		})
	}
	return WidgetProperties{
		View:    viewTimeSeries,
		Stacked: false,
		Period:  Period,
		Title:   "Response Errors",
		Region:  region,
		Metrics: metrics,
		YAxis: &YAxis{
			Left: &YAxisSide{
				ShowUnits: false,
			},
		},
	}
}

func getLambdaMetric(functionName, metric, color, stat string) []any {
	return []any{
		"AWS/Lambda", metric, "FunctionName", functionName,
		map[string]string{keyStat: stat, keyColor: color, keyLabel: metric},
	}
}

func AmplifyRequestsMetrics(region string, amplifyApps []resource.AmplifyApp) WidgetProperties {
	metrics := make([][]any, 0, len(amplifyApps))
	for i, app := range amplifyApps {
		color := colorPalette[i%len(colorPalette)]
		metrics = append(metrics, getAmplifyMetricForApp(app.ID, app.Name, "Requests", color, statSum))
	}
	return WidgetProperties{
		View:    viewTimeSeries,
		Stacked: false,
		Period:  Period,
		Title:   "Requests",
		Region:  region,
		Metrics: metrics,
		YAxis: &YAxis{
			Left: &YAxisSide{
				ShowUnits: false,
			},
		},
	}
}

func AmplifyErrorsMetrics4xx(region string, amplifyApps []resource.AmplifyApp) WidgetProperties {
	metrics := make([][]any, 0, len(amplifyApps))
	for i, app := range amplifyApps {
		color := colorPalette[i%len(colorPalette)]
		metrics = append(metrics, getAmplifyMetricForApp(app.ID, app.Name, "4xxErrors", color, statSum))
	}
	return WidgetProperties{
		View:    viewTimeSeries,
		Stacked: false,
		Period:  Period,
		Title:   "4xx Errors",
		Region:  region,
		Metrics: metrics,
		YAxis: &YAxis{
			Left: &YAxisSide{
				ShowUnits: false,
			},
		},
	}
}

func AmplifyErrorsMetrics5xx(region string, amplifyApps []resource.AmplifyApp) WidgetProperties {
	metrics := make([][]any, 0, len(amplifyApps))
	for i, app := range amplifyApps {
		color := colorPalette[i%len(colorPalette)]
		metrics = append(metrics, getAmplifyMetricForApp(app.ID, app.Name, "5xxErrors", color, statSum))
	}
	return WidgetProperties{
		View:    viewTimeSeries,
		Stacked: false,
		Period:  Period,
		Title:   "5xx Errors",
		Region:  region,
		Metrics: metrics,
		YAxis: &YAxis{
			Left: &YAxisSide{
				ShowUnits: false,
			},
		},
	}
}

func getAmplifyMetricForApp(appId, appName, metric, color, stat string) []any {
	return []any{
		"AWS/AmplifyHosting", metric, "App", appId,
		map[string]string{keyStat: stat, keyColor: color, keyLabel: appName},
	}
}

// Deprecated: Use AmplifyRequestsMetrics, AmplifyErrorsMetrics4xx, and AmplifyErrorsMetrics5xx instead.
func AmplifyMetrics(region, appId, title string) WidgetProperties {
	return WidgetProperties{
		View:    viewTimeSeries,
		Stacked: false,
		Period:  Period,
		Title:   title,
		Region:  region,
		Metrics: [][]any{
			getAmplifyMetric(appId, "Requests", colorDarkBlue, statSum),
			getAmplifyMetric(appId, "4xxErrors", colorLightRed, statSum),
			getAmplifyMetric(appId, "5xxErrors", colorDarkRed, statSum),
		},
		YAxis: &YAxis{
			Left: &YAxisSide{
				ShowUnits: false,
			},
		},
	}
}

func getAmplifyMetric(appId, metric, color, stat string) []any {
	return []any{
		"AWS/AmplifyHosting", metric, "App", appId,
		map[string]string{keyStat: stat, keyColor: color, keyLabel: metric},
	}
}

func S3BucketMetrics(region, bucketName, title string) WidgetProperties {
	return WidgetProperties{
		View:    viewTimeSeries,
		Stacked: false,
		Period:  Period,
		Title:   title,
		Region:  region,
		Metrics: [][]any{
			getS3BucketMetric(bucketName, "BucketSizeBytes", "StandardStorage"),
			getS3BucketMetric(bucketName, "NumberOfObjects", "AllStorageTypes"),
		},
		YAxis: &YAxis{
			Left: &YAxisSide{
				ShowUnits: false,
			},
		},
	}
}

func getS3BucketMetric(bucketName, metric, storageType string) []any {
	return []any{
		"AWS/S3", metric, "BucketName", bucketName, "StorageType", storageType,
		map[string]int{"period": 86400},
	}
}

func S3BucketSizeMetrics(region string, buckets []string) WidgetProperties {
	metrics := make([][]any, 0, len(buckets))
	for i, b := range buckets {
		color := colorPalette[i%len(colorPalette)]
		metrics = append(metrics, getS3BucketMetricForBucket(b, b, "BucketSizeBytes", "StandardStorage", color))
	}
	return WidgetProperties{
		View:    viewTimeSeries,
		Stacked: false,
		Period:  Period,
		Title:   "Bucket Size",
		Region:  region,
		Metrics: metrics,
		YAxis: &YAxis{
			Left: &YAxisSide{
				Label:     "",
				ShowUnits: true,
			},
		},
	}
}

func S3ObjectCountMetrics(region string, buckets []string) WidgetProperties {
	metrics := make([][]any, 0, len(buckets))
	for i, b := range buckets {
		color := colorPalette[i%len(colorPalette)]
		metrics = append(metrics, getS3BucketMetricForBucket(b, b, "NumberOfObjects", "AllStorageTypes", color))
	}
	return WidgetProperties{
		View:    viewTimeSeries,
		Stacked: false,
		Period:  Period,
		Title:   "Number of Objects",
		Region:  region,
		Metrics: metrics,
		YAxis: &YAxis{
			Left: &YAxisSide{
				ShowUnits: false,
			},
		},
	}
}

func getS3BucketMetricForBucket(bucketName, bucketLabel, metric, storageType, color string) []any {
	return []any{
		"AWS/S3", metric, "BucketName", bucketName, "StorageType", storageType,
		map[string]any{
			"period": 86400,
			keyColor: color,
			keyLabel: bucketLabel,
		},
	}
}

func DynamoDBMetrics(region, tableName, title string) WidgetProperties {
	return WidgetProperties{
		View:    viewTimeSeries,
		Stacked: false,
		Period:  Period,
		Title:   title,
		Region:  region,
		Stat:    statAvg,
		Metrics: [][]any{
			getDynamoDBMetricsHidden(region, tableName, "ConsumedReadCapacityUnits", "m1"),
			getDynamoDBMetricsCalculate(
				region, "m1/PERIOD(m1)", "Read usage (average units/second)", "e1", colorLightBlue,
			),
			getDynamoDBMetricsHidden(region, tableName, "ConsumedWriteCapacityUnits", "m2"),
			getDynamoDBMetricsCalculate(
				region, "m2/PERIOD(m2)", "Write usage (average units/second)", "e2", colorDarkBlue,
			),
			// Other types of read operations you can get errors for: Scan, Query, BatchGetItem, TransactGetItems.
			getDynamoDBMetrics(region, tableName, "SystemErrors", "GetItem", colorLightRed),
			// Other types of write operations you can get errors for: UpdateItem, DeleteItem, BatchWriteItem, TransactWriteItems.
			getDynamoDBMetrics(region, tableName, "SystemErrors", "PutItem", colorDarkRed),
		},
		YAxis: &YAxis{
			Left: &YAxisSide{
				ShowUnits: false,
			},
		},
	}
}

func DynamoDBReadUsageMetrics(region string, tableNames []string) WidgetProperties {
	metrics := make([][]any, 0, len(tableNames)*2)
	for i, tableName := range tableNames {
		color := colorPalette[i%len(colorPalette)]
		hiddenID := fmt.Sprintf("mread%d", i)
		expressionID := fmt.Sprintf("eread%d", i)
		expression := fmt.Sprintf("%s/PERIOD(%s)", hiddenID, hiddenID)

		metrics = append(metrics, getDynamoDBMetricsHidden(region, tableName, "ConsumedReadCapacityUnits", hiddenID))
		metrics = append(metrics, getDynamoDBMetricsCalculate(region, expression, tableName, expressionID, color))
	}

	return WidgetProperties{
		View:    viewTimeSeries,
		Stacked: false,
		Period:  Period,
		Title:   "Read Usage (average units/second)",
		Region:  region,
		Stat:    statAvg,
		Metrics: metrics,
		YAxis: &YAxis{
			Left: &YAxisSide{
				ShowUnits: false,
			},
		},
	}
}

func DynamoDBWriteUsageMetrics(region string, tableNames []string) WidgetProperties {
	metrics := make([][]any, 0, len(tableNames)*2)
	for i, tableName := range tableNames {
		color := colorPalette[i%len(colorPalette)]
		hiddenID := fmt.Sprintf("mwrite%d", i)
		expressionID := fmt.Sprintf("ewrite%d", i)
		expression := fmt.Sprintf("%s/PERIOD(%s)", hiddenID, hiddenID)

		metrics = append(metrics, getDynamoDBMetricsHidden(region, tableName, "ConsumedWriteCapacityUnits", hiddenID))
		metrics = append(metrics, getDynamoDBMetricsCalculate(region, expression, tableName, expressionID, color))
	}

	return WidgetProperties{
		View:    viewTimeSeries,
		Stacked: false,
		Period:  Period,
		Title:   "Write Usage (average units/second)",
		Region:  region,
		Stat:    statAvg,
		Metrics: metrics,
		YAxis: &YAxis{
			Left: &YAxisSide{
				ShowUnits: false,
			},
		},
	}
}

func DynamoDBReadSystemErrorsMetrics(region string, tableNames []string) WidgetProperties {
	metrics := make([][]any, 0, len(tableNames))
	for i, tableName := range tableNames {
		color := colorPalette[i%len(colorPalette)]
		metrics = append(metrics, getDynamoDBSystemErrorsMetricForTable(region, tableName, tableName, "GetItem", color))
	}

	return WidgetProperties{
		View:    viewTimeSeries,
		Stacked: false,
		Period:  Period,
		Title:   "Read System Errors",
		Region:  region,
		Stat:    statSum,
		Metrics: metrics,
		YAxis: &YAxis{
			Left: &YAxisSide{
				ShowUnits: false,
			},
		},
	}
}

func DynamoDBWriteSystemErrorsMetrics(region string, tableNames []string) WidgetProperties {
	metrics := make([][]any, 0, len(tableNames))
	for i, tableName := range tableNames {
		color := colorPalette[i%len(colorPalette)]
		metrics = append(metrics, getDynamoDBSystemErrorsMetricForTable(region, tableName, tableName, "PutItem", color))
	}

	return WidgetProperties{
		View:    viewTimeSeries,
		Stacked: false,
		Period:  Period,
		Title:   "Write System Errors",
		Region:  region,
		Stat:    statSum,
		Metrics: metrics,
		YAxis: &YAxis{
			Left: &YAxisSide{
				ShowUnits: false,
			},
		},
	}
}

// getDynamoDBMetricsHidden gets the raw data with Sum stat. The data is not
// shown in the widget, it's meant to be used in an expression to calculate
// another metric.
func getDynamoDBMetricsHidden(region, tableName, metric, id string) []any {
	return []any{
		"AWS/DynamoDB", metric, "TableName", tableName,
		map[string]any{
			keyStat:   statSum,
			"id":      id,
			keyRegion: region,
			"visible": false,
		},
	}
}

// getDynamoDBMetricsCalculate calculates a metric using an expression and data
// from a hidden metric that has id.
func getDynamoDBMetricsCalculate(region, expression, label, id, color string) []any {
	return []any{
		map[string]any{
			"expression": expression,
			keyLabel:     label,
			"id":         id,
			keyColor:     color,
			keyRegion:    region,
		},
	}
}

func getDynamoDBMetrics(region, tableName, metric, operation, color string) []any {
	return []any{
		"AWS/DynamoDB", metric, "TableName", tableName, "Operation", operation,
		map[string]string{keyColor: color, keyRegion: region},
	}
}

func getDynamoDBSystemErrorsMetricForTable(region, tableName, tableLabel, operation, color string) []any {
	return []any{
		"AWS/DynamoDB", "SystemErrors", "TableName", tableName, "Operation", operation,
		map[string]string{keyColor: color, keyRegion: region, keyLabel: tableLabel, keyStat: statSum},
	}
}

func ApiGatewayMetrics(region, apiName, title string) WidgetProperties {
	return WidgetProperties{
		View:    viewTimeSeries,
		Stacked: false,
		Period:  Period,
		Title:   title,
		Region:  region,
		Metrics: [][]any{
			getApiGatewayMetric(apiName, "Count", colorDarkBlue, statSum),
			getApiGatewayMetric(apiName, "4XXError", colorLightRed, statSum),
			getApiGatewayMetric(apiName, "5XXError", colorDarkRed, statSum),
		},
		YAxis: &YAxis{
			Left: &YAxisSide{
				ShowUnits: false,
			},
		},
	}
}

func getApiGatewayMetric(apiName, metric, color, stat string) []any {
	return []any{
		"AWS/ApiGateway", metric, "ApiName", apiName,
		map[string]string{keyStat: stat, keyColor: color, keyLabel: metric},
	}
}

func ApiGatewayCountMetrics(region string, apis []string) WidgetProperties {
	metrics := make([][]any, 0, len(apis))
	for i, api := range apis {
		color := colorPalette[i%len(colorPalette)]
		metrics = append(metrics, getApiGatewayMetricForAPI(api, api, "Count", color, statSum))
	}
	return WidgetProperties{
		View:    viewTimeSeries,
		Stacked: false,
		Period:  Period,
		Title:   "Count",
		Region:  region,
		Metrics: metrics,
		YAxis: &YAxis{
			Left: &YAxisSide{
				ShowUnits: false,
			},
		},
	}
}

func ApiGatewayErrorsMetrics4xx(region string, apis []string) WidgetProperties {
	metrics := make([][]any, 0, len(apis))
	for i, api := range apis {
		color := colorPalette[i%len(colorPalette)]
		metrics = append(metrics, getApiGatewayMetricForAPI(api, api, "4XXError", color, statSum))
	}
	return WidgetProperties{
		View:    viewTimeSeries,
		Stacked: false,
		Period:  Period,
		Title:   "4xx Errors",
		Region:  region,
		Metrics: metrics,
		YAxis: &YAxis{
			Left: &YAxisSide{
				ShowUnits: false,
			},
		},
	}
}

func ApiGatewayErrorsMetrics5xx(region string, apis []string) WidgetProperties {
	metrics := make([][]any, 0, len(apis))
	for i, api := range apis {
		color := colorPalette[i%len(colorPalette)]
		metrics = append(metrics, getApiGatewayMetricForAPI(api, api, "5XXError", color, statSum))
	}
	return WidgetProperties{
		View:    viewTimeSeries,
		Stacked: false,
		Period:  Period,
		Title:   "5xx Errors",
		Region:  region,
		Metrics: metrics,
		YAxis: &YAxis{
			Left: &YAxisSide{
				ShowUnits: false,
			},
		},
	}
}

func getApiGatewayMetricForAPI(apiName, apiLabel, metric, color, stat string) []any {
	return []any{
		"AWS/ApiGateway", metric, "ApiName", apiName,
		map[string]string{keyStat: stat, keyColor: color, keyLabel: apiLabel},
	}
}

func SQSMetrics(region, queueName, title string) WidgetProperties {
	return WidgetProperties{
		View:    viewTimeSeries,
		Stacked: false,
		Period:  Period,
		Title:   title,
		Region:  region,
		Metrics: [][]any{
			getSQSMetric(queueName, "NumberOfMessagesReceived", colorDarkGreen, statSum),
			getSQSMetric(queueName, "NumberOfMessagesSent", colorDarkBlue, statSum),
			getSQSMetric(queueName, "NumberOfMessagesDeleted", colorLightGreen, statSum),
			getSQSMetric(queueName, "NumberOfEmptyReceives", colorLightRed, statSum),
			getSQSMetric(queueName, "ApproximateNumberOfMessagesVisible", colorLightBlue, statAvg),
		},
		YAxis: &YAxis{
			Left: &YAxisSide{
				ShowUnits: false,
			},
		},
	}
}

func getSQSMetric(queueName, metric, color, stat string) []any {
	return []any{
		"AWS/SQS", metric, "QueueName", queueName,
		map[string]string{keyStat: stat, keyColor: color, keyLabel: metric},
	}
}

func SQSReceivedMetrics(region string, queueNames []string) WidgetProperties {
	metrics := make([][]any, 0, len(queueNames))
	for i, q := range queueNames {
		color := colorPalette[i%len(colorPalette)]
		metrics = append(metrics, getSQSMetricForQueue(q, q, "NumberOfMessagesReceived", color, statSum))
	}
	return WidgetProperties{
		View:    viewTimeSeries,
		Stacked: false,
		Period:  Period,
		Title:   "Messages Received",
		Region:  region,
		Metrics: metrics,
		YAxis: &YAxis{
			Left: &YAxisSide{
				ShowUnits: false,
			},
		},
	}
}

func SQSSentMetrics(region string, queueNames []string) WidgetProperties {
	metrics := make([][]any, 0, len(queueNames))
	for i, q := range queueNames {
		color := colorPalette[i%len(colorPalette)]
		metrics = append(metrics, getSQSMetricForQueue(q, q, "NumberOfMessagesSent", color, statSum))
	}
	return WidgetProperties{
		View:    viewTimeSeries,
		Stacked: false,
		Period:  Period,
		Title:   "Messages Sent",
		Region:  region,
		Metrics: metrics,
		YAxis: &YAxis{
			Left: &YAxisSide{
				ShowUnits: false,
			},
		},
	}
}

func SQSDeletedMetrics(region string, queueNames []string) WidgetProperties {
	metrics := make([][]any, 0, len(queueNames))
	for i, q := range queueNames {
		color := colorPalette[i%len(colorPalette)]
		metrics = append(metrics, getSQSMetricForQueue(q, q, "NumberOfMessagesDeleted", color, statSum))
	}
	return WidgetProperties{
		View:    viewTimeSeries,
		Stacked: false,
		Period:  Period,
		Title:   "Messages Deleted",
		Region:  region,
		Metrics: metrics,
		YAxis: &YAxis{
			Left: &YAxisSide{
				ShowUnits: false,
			},
		},
	}
}

func SQSEmptyReceivesMetrics(region string, queueNames []string) WidgetProperties {
	metrics := make([][]any, 0, len(queueNames))
	for i, q := range queueNames {
		color := colorPalette[i%len(colorPalette)]
		metrics = append(metrics, getSQSMetricForQueue(q, q, "NumberOfEmptyReceives", color, statSum))
	}
	return WidgetProperties{
		View:    viewTimeSeries,
		Stacked: false,
		Period:  Period,
		Title:   "Empty Receives",
		Region:  region,
		Metrics: metrics,
		YAxis: &YAxis{
			Left: &YAxisSide{
				ShowUnits: false,
			},
		},
	}
}

func SQSVisibleMetrics(region string, queueNames []string) WidgetProperties {
	metrics := make([][]any, 0, len(queueNames))
	for i, q := range queueNames {
		color := colorPalette[i%len(colorPalette)]
		metrics = append(metrics, getSQSMetricForQueue(q, q, "ApproximateNumberOfMessagesVisible", color, statAvg))
	}
	return WidgetProperties{
		View:    viewTimeSeries,
		Stacked: false,
		Period:  Period,
		Title:   "Approx Visible Messages",
		Region:  region,
		Metrics: metrics,
		YAxis: &YAxis{
			Left: &YAxisSide{
				ShowUnits: false,
			},
		},
	}
}

func getSQSMetricForQueue(queueName, queueLabel, metric, color, stat string) []any {
	return []any{
		"AWS/SQS", metric, "QueueName", queueName,
		map[string]string{keyStat: stat, keyColor: color, keyLabel: queueLabel},
	}
}

func ECSClusterMetrics(region, clusterName, title string) WidgetProperties {
	return WidgetProperties{
		View:    viewTimeSeries,
		Stacked: false,
		Period:  Period,
		Title:   title,
		Region:  region,
		Metrics: [][]any{
			getECSClusterMetric(clusterName, "CPUUtilization", colorDarkBlue, statAvg),
			getECSClusterMetric(clusterName, "MemoryUtilization", colorDarkGreen, statAvg),
		},
		YAxis: &YAxis{
			Left: &YAxisSide{
				ShowUnits: false,
			},
		},
	}
}

func getECSClusterMetric(clusterName, metric, color, stat string) []any {
	return []any{
		"AWS/ECS", metric, "ClusterName", clusterName,
		map[string]string{keyStat: stat, keyColor: color, keyLabel: metric},
	}
}

func ECSCPUMetrics(region string, clusterNames []string) WidgetProperties {
	metrics := make([][]any, 0, len(clusterNames))
	for i, c := range clusterNames {
		color := colorPalette[i%len(colorPalette)]
		metrics = append(metrics, getECSClusterMetricForCluster(c, c, "CPUUtilization", color, statAvg))
	}
	return WidgetProperties{
		View:    viewTimeSeries,
		Stacked: false,
		Period:  Period,
		Title:   "CPU Utilization",
		Region:  region,
		Metrics: metrics,
		YAxis: &YAxis{
			Left: &YAxisSide{
				ShowUnits: false,
			},
		},
	}
}

func ECSMemoryMetrics(region string, clusterNames []string) WidgetProperties {
	metrics := make([][]any, 0, len(clusterNames))
	for i, c := range clusterNames {
		color := colorPalette[i%len(colorPalette)]
		metrics = append(metrics, getECSClusterMetricForCluster(c, c, "MemoryUtilization", color, statAvg))
	}
	return WidgetProperties{
		View:    viewTimeSeries,
		Stacked: false,
		Period:  Period,
		Title:   "Memory Utilization",
		Region:  region,
		Metrics: metrics,
		YAxis: &YAxis{
			Left: &YAxisSide{
				ShowUnits: false,
			},
		},
	}
}

func getECSClusterMetricForCluster(clusterName, clusterLabel, metric, color, stat string) []any {
	return []any{
		"AWS/ECS", metric, "ClusterName", clusterName,
		map[string]string{keyStat: stat, keyColor: color, keyLabel: clusterLabel},
	}
}

func EventBusMetrics(region, eventBusName, title string) WidgetProperties {
	return WidgetProperties{
		View:    viewTimeSeries,
		Stacked: false,
		Period:  300,
		Title:   title,
		Region:  region,
		Metrics: [][]any{
			getEventBusMetric(eventBusName, "Invocations", colorLightBlue, statSum),
		},
		YAxis: &YAxis{
			Left: &YAxisSide{
				ShowUnits: false,
			},
		},
	}
}

func getEventBusMetric(eventBusName, metric, color, stat string) []any {
	return []any{
		"AWS/Events", metric, "EventBusName", eventBusName,
		map[string]string{keyStat: stat, keyColor: color, keyLabel: metric},
	}
}

func EventBusInvocationsMetrics(region string, eventBuses []string) WidgetProperties {
	metrics := make([][]any, 0, len(eventBuses))
	for i, eb := range eventBuses {
		color := colorPalette[i%len(colorPalette)]
		metrics = append(metrics, getEventBusMetricForBus(eb, eb, "Invocations", color, statSum))
	}
	return WidgetProperties{
		View:    viewTimeSeries,
		Stacked: false,
		Period:  300,
		Title:   "Invocations",
		Region:  region,
		Metrics: metrics,
		YAxis: &YAxis{
			Left: &YAxisSide{
				ShowUnits: false,
			},
		},
	}
}

func getEventBusMetricForBus(eventBusName, eventBusLabel, metric, color, stat string) []any {
	return []any{
		"AWS/Events", metric, "EventBusName", eventBusName,
		map[string]string{keyStat: stat, keyColor: color, keyLabel: eventBusLabel},
	}
}
