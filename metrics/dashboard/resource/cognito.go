package resource

import (
	"context"
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

const (
	cognitoAPIConcurrencyLimit = 10
)

// CognitoUserPool holds user pool ID and its associated client IDs
type CognitoUserPool struct {
	ID        string
	ClientIDs []string
}

func GetCognitoUserPools(ctx context.Context, cfg aws.Config, tags map[string]string) ([]CognitoUserPool, error) {
	client := cognitoidentityprovider.NewFromConfig(cfg)

	// First get all user pools
	allPools, err := getCognitoUserPools(ctx, client)
	if err != nil {
		return nil, err
	}

	// Get account ID once for all pools
	accountID, err := getAccountID(ctx, cfg)
	if err != nil {
		return nil, err
	}

	// Process pools concurrently
	type poolResult struct {
		pool CognitoUserPool
		err  error
	}

	results := make(chan poolResult, len(allPools))
	var wg sync.WaitGroup

	semaphore := make(chan struct{}, cognitoAPIConcurrencyLimit)

	for _, pool := range allPools {
		wg.Add(1)
		go func(pool types.UserPoolDescriptionType) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			poolID := *pool.Id

			// Get tags for this user pool
			poolTags, err := getUserPoolTagsWithAccountID(ctx, cfg, poolID, accountID)
			if err != nil {
				results <- poolResult{err: fmt.Errorf("getting pool tags: %w", err)}
				return
			}

			// Check if the pool has the required tag
			if poolTags != nil {
				if matchesAllTags(poolTags, tags) {
					clientIDs, err := getUserPoolClientIDs(ctx, client, poolID)
					if err != nil {
						results <- poolResult{err: fmt.Errorf("getting pool client IDs: %w", err)}
						return
					}

					results <- poolResult{
						pool: CognitoUserPool{
							ID:        poolID,
							ClientIDs: clientIDs,
						},
					}
					return
				}
			}

			// No error, but pool doesn't match criteria
			results <- poolResult{}
		}(pool)
	}

	// Close results channel when all goroutines are done
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	var poolInfos []CognitoUserPool
	for result := range results {
		if result.err != nil {
			return nil, result.err
		}
		if result.pool.ID != "" {
			poolInfos = append(poolInfos, result.pool)
		}
	}

	return poolInfos, nil
}

func getCognitoUserPools(
	ctx context.Context,
	client *cognitoidentityprovider.Client,
) ([]types.UserPoolDescriptionType, error) {
	var allPools []types.UserPoolDescriptionType

	input := &cognitoidentityprovider.ListUserPoolsInput{
		MaxResults: aws.Int32(60), // Maximum allowed by API
	}

	paginator := cognitoidentityprovider.NewListUserPoolsPaginator(client, input)

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("error listing user pools: %w", err)
		}
		allPools = append(allPools, output.UserPools...)
	}

	return allPools, nil
}

func getAccountID(ctx context.Context, cfg aws.Config) (string, error) {
	stsClient := sts.NewFromConfig(cfg)
	identity, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return "", fmt.Errorf("error getting caller identity: %w", err)
	}
	return *identity.Account, nil
}

func getUserPoolTagsWithAccountID(
	ctx context.Context,
	cfg aws.Config,
	userPoolId, accountID string,
) (map[string]string, error) {
	region := cfg.Region

	// Create Cognito client for this specific call
	client := cognitoidentityprovider.NewFromConfig(cfg)

	input := &cognitoidentityprovider.ListTagsForResourceInput{
		ResourceArn: aws.String(fmt.Sprintf("arn:aws:cognito-idp:%s:%s:userpool/%s", region, accountID, userPoolId)),
	}

	result, err := client.ListTagsForResource(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("error getting tags for user pool %s: %w", userPoolId, err)
	}

	return result.Tags, nil
}

func getUserPoolClientIDs(
	ctx context.Context,
	client *cognitoidentityprovider.Client,
	userPoolId string,
) ([]string, error) {
	var clientIDs []string

	input := &cognitoidentityprovider.ListUserPoolClientsInput{
		UserPoolId: aws.String(userPoolId),
		MaxResults: aws.Int32(60), // Maximum allowed by API
	}

	paginator := cognitoidentityprovider.NewListUserPoolClientsPaginator(client, input)

	for paginator.HasMorePages() {
		result, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("error listing user pool clients for pool %s: %w", userPoolId, err)
		}
		for _, client := range result.UserPoolClients {
			clientIDs = append(clientIDs, *client.ClientId)
		}
	}

	return clientIDs, nil
}
