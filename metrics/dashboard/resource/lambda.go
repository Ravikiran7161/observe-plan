package resource

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
)

func GetLambdaFunctions(ctx context.Context, cfg aws.Config, tags map[string]string) ([]string, error) {
	arns, err := getARNs(ctx, cfg, []string{"lambda:function"}, tags)
	if err != nil {
		return nil, err
	}

	functions := make([]string, 0, len(arns))
	for _, arn := range arns {
		function, err := extractFunctionName(arn)
		if err != nil {
			return nil, err
		}
		functions = append(functions, function)
	}
	return functions, nil
}

// extractFunctionName extracts the function name from a Lambda function ARN
// that has the following format: arn:aws:lambda:region:account:function:function-name
func extractFunctionName(arnStr string) (string, error) {
	parsedArn, err := arn.Parse(arnStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse arn: %w", err)
	}

	if parsedArn.Service != "lambda" {
		return "", fmt.Errorf("not lambda function arn: %s", arnStr)
	}

	// Resource format: "function:function-name" or "function:function-name:version"
	parts := strings.Split(parsedArn.Resource, ":")
	if len(parts) < 2 || parts[0] != "function" {
		return "", fmt.Errorf("invalid lambda function resource format: %s", arnStr)
	}

	return parts[1], nil
}
