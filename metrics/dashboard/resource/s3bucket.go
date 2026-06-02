package resource

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
)

func GetS3Buckets(ctx context.Context, cfg aws.Config, tags map[string]string) ([]string, error) {
	arns, err := getARNs(ctx, cfg, []string{"s3"}, tags)
	if err != nil {
		return nil, err
	}

	buckets := make([]string, 0, len(arns))
	for _, arn := range arns {
		bucket, err := extractBucketName(arn)
		if err != nil {
			return nil, err
		}
		buckets = append(buckets, bucket)
	}

	return buckets, nil
}

func extractBucketName(arnStr string) (string, error) {
	// Bucket: arn:aws:s3:::bucket_name
	parsedArn, err := arn.Parse(arnStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse arn: %w", err)
	}

	if parsedArn.Service != "s3" {
		return "", fmt.Errorf("not s3 arn: %s", arnStr)
	}

	resource := parsedArn.Resource
	if resource == "" {
		return "", fmt.Errorf("empty bucket name in arn: %s", arnStr)
	}

	// Object: arn:aws:s3:::bucket_name/object_key
	if idx := strings.Index(resource, "/"); idx >= 0 {
		resource = resource[:idx]
	}

	return resource, nil
}
