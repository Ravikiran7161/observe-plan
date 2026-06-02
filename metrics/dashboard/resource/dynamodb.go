package resource

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
)

func GetDynamoDBTables(ctx context.Context, cfg aws.Config, tags map[string]string) ([]string, error) {
	arns, err := getARNs(ctx, cfg, []string{"dynamodb:table"}, tags)
	if err != nil {
		return nil, err
	}

	tables := make([]string, 0, len(arns))
	for _, arnStr := range arns {
		table, err := extractNameFromARN(arnStr, "dynamodb", "table")
		if err != nil {
			return nil, err
		}
		tables = append(tables, table)
	}

	return tables, nil
}
