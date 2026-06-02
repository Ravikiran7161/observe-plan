package dashboard

import "github.com/thermofisher/cmd-saas/observe-plane/metrics/dashboard/resource"

const Period = 60

type Widget struct {
	Type       string `json:"type"`
	Height     int    `json:"height"`
	Width      int    `json:"width"`
	Properties any    `json:"properties"`
}

func NewTextWidget(markdown string, width, height int) Widget {
	return Widget{
		Type:       "text",
		Width:      width,
		Height:     height,
		Properties: map[string]string{"markdown": markdown},
	}
}

func NewMetricWidget(properties WidgetProperties, width, height int) Widget {
	return Widget{
		Type:       "metric",
		Width:      width,
		Height:     height,
		Properties: properties,
	}
}

type WidgetProperties struct {
	View    string  `json:"view"`
	Stacked bool    `json:"stacked"`
	Region  string  `json:"region"`
	Period  int     `json:"period"`
	Title   string  `json:"title"`
	Metrics [][]any `json:"metrics"`
	YAxis   *YAxis  `json:"yAxis,omitempty"`
	Stat    string  `json:"stat,omitempty"`
}

type YAxis struct {
	Left *YAxisSide `json:"left,omitempty"`
}

type YAxisSide struct {
	Label     string `json:"label,omitempty"`
	ShowUnits bool   `json:"showUnits"`
}

func addAmplifyWidgets(widgets []Widget, region string, amplifyApps []resource.AmplifyApp) []Widget {
	if len(amplifyApps) == 0 {
		return widgets
	}
	widgets = append(widgets, NewTextWidget("## Amplify Apps", 24, 1))
	// for _, app := range amplifyApps {
	// 	widgets = append(widgets, NewMetricWidget(AmplifyMetrics(region, app.ID, app.Name), 8, 5))
	// }
	widgets = append(widgets, NewMetricWidget(AmplifyRequestsMetrics(region, amplifyApps), 8, 5))
	widgets = append(widgets, NewMetricWidget(AmplifyErrorsMetrics4xx(region, amplifyApps), 8, 5))
	widgets = append(widgets, NewMetricWidget(AmplifyErrorsMetrics5xx(region, amplifyApps), 8, 5))
	return widgets
}

func addAPIGatewayWidgets(widgets []Widget, region string, apiGateways []string) []Widget {
	if len(apiGateways) == 0 {
		return widgets
	}
	widgets = append(widgets, NewTextWidget("## API Gateways", 24, 1))
	// for _, api := range apiGateways {
	// 	widgets = append(widgets, NewMetricWidget(ApiGatewayMetrics(region, api, api), 8, 5))
	// }
	widgets = append(widgets, NewMetricWidget(ApiGatewayCountMetrics(region, apiGateways), 8, 5))
	widgets = append(widgets, NewMetricWidget(ApiGatewayErrorsMetrics4xx(region, apiGateways), 8, 5))
	widgets = append(widgets, NewMetricWidget(ApiGatewayErrorsMetrics5xx(region, apiGateways), 8, 5))
	return widgets
}

func addCognitoWidgets(widgets []Widget, region string, cognitoUserPools []resource.CognitoUserPool) []Widget {
	if len(cognitoUserPools) == 0 {
		return widgets
	}
	widgets = append(widgets, NewTextWidget("## Cognito User Pools", 24, 1))
	// for _, userPool := range cognitoUserPools {
	// 	for _, clientID := range userPool.ClientIDs {
	// 		metrics := CognitoMetrics(region, userPool.ID, clientID, userPool.ID)
	// 		widgets = append(widgets, NewMetricWidget(metrics, 8, 5))
	// 	}
	// }
	widgets = append(widgets, NewMetricWidget(CognitoSignUpMetrics(region, cognitoUserPools), 8, 5))
	widgets = append(widgets, NewMetricWidget(CognitoSignInMetrics(region, cognitoUserPools), 8, 5))
	widgets = append(widgets, NewMetricWidget(CognitoTokenRefreshMetrics(region, cognitoUserPools), 8, 5))
	return widgets
}

func addSQSWidgets(widgets []Widget, region string, queueNames []string) []Widget {
	if len(queueNames) == 0 {
		return widgets
	}
	widgets = append(widgets, NewTextWidget("## SQS Queues", 24, 1))
	// for _, queueName := range queueNames {
	// 	widgets = append(widgets, NewMetricWidget(SQSMetrics(region, queueName, queueName), 8, 5))
	// }
	widgets = append(widgets, NewMetricWidget(SQSReceivedMetrics(region, queueNames), 8, 5))
	widgets = append(widgets, NewMetricWidget(SQSSentMetrics(region, queueNames), 8, 5))
	widgets = append(widgets, NewMetricWidget(SQSDeletedMetrics(region, queueNames), 8, 5))
	widgets = append(widgets, NewMetricWidget(SQSEmptyReceivesMetrics(region, queueNames), 8, 5))
	widgets = append(widgets, NewMetricWidget(SQSVisibleMetrics(region, queueNames), 8, 5))
	return widgets
}

func addLambdaWidgets(widgets []Widget, region string, functions []string) []Widget {
	if len(functions) == 0 {
		return widgets
	}
	widgets = append(widgets, NewTextWidget("## Lambda Functions", 24, 1))
	// for _, functionName := range functions {
	// 	widgets = append(widgets, NewMetricWidget(LambdaMetrics(region, functionName, functionName), 8, 5))
	// }
	widgets = append(widgets, NewMetricWidget(LambdaInvocationsMetrics(region, functions), 8, 5))
	widgets = append(widgets, NewMetricWidget(LambdaErrorsMetrics(region, functions), 8, 5))
	widgets = append(widgets, NewMetricWidget(LambdaResponseErrorsMetrics(region, functions), 8, 5))
	return widgets
}

func addDynamoDBWidgets(widgets []Widget, region string, dynamoDBTables []string) []Widget {
	if len(dynamoDBTables) == 0 {
		return widgets
	}
	widgets = append(widgets, NewTextWidget("## DynamoDB Tables", 24, 1))
	// for _, table := range dynamoDBTables {
	// 	widgets = append(widgets, NewMetricWidget(DynamoDBMetrics(region, table, table), 8, 5))
	// }
	widgets = append(widgets, NewMetricWidget(DynamoDBReadUsageMetrics(region, dynamoDBTables), 8, 5))
	widgets = append(widgets, NewMetricWidget(DynamoDBWriteUsageMetrics(region, dynamoDBTables), 8, 5))
	widgets = append(widgets, NewMetricWidget(DynamoDBReadSystemErrorsMetrics(region, dynamoDBTables), 8, 5))
	widgets = append(widgets, NewMetricWidget(DynamoDBWriteSystemErrorsMetrics(region, dynamoDBTables), 8, 5))
	return widgets
}

func addS3Widgets(widgets []Widget, region string, s3Buckets []string) []Widget {
	if len(s3Buckets) == 0 {
		return widgets
	}
	widgets = append(widgets, NewTextWidget("## S3 Buckets", 24, 1))
	// for _, bucket := range s3Buckets {
	// 	widgets = append(widgets, NewMetricWidget(S3BucketMetrics(region, bucket, bucket), 8, 5))
	// }
	widgets = append(widgets, NewMetricWidget(S3BucketSizeMetrics(region, s3Buckets), 8, 5))
	widgets = append(widgets, NewMetricWidget(S3ObjectCountMetrics(region, s3Buckets), 8, 5))
	return widgets
}

func addECSClusterWidgets(widgets []Widget, region string, ecsClusters []string) []Widget {
	if len(ecsClusters) == 0 {
		return widgets
	}
	widgets = append(widgets, NewTextWidget("## ECS Clusters", 24, 1))
	// for _, cluster := range ecsClusters {
	// 	widgets = append(widgets, NewMetricWidget(ECSClusterMetrics(region, cluster, cluster), 8, 5))
	// }
	widgets = append(widgets, NewMetricWidget(ECSCPUMetrics(region, ecsClusters), 8, 5))
	widgets = append(widgets, NewMetricWidget(ECSMemoryMetrics(region, ecsClusters), 8, 5))
	return widgets
}

func addEventBusWidgets(widgets []Widget, region string, eventBuses []string) []Widget {
	if len(eventBuses) == 0 {
		return widgets
	}
	widgets = append(widgets, NewTextWidget("## EventBridge Event Buses", 24, 1))
	// for _, eventBus := range eventBuses {
	// 	widgets = append(widgets, NewMetricWidget(EventBusMetrics(region, eventBus, eventBus), 8, 5))
	// }
	widgets = append(widgets, NewMetricWidget(EventBusInvocationsMetrics(region, eventBuses), 8, 5))
	return widgets
}
