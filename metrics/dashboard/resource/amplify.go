package resource

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/amplify"
)

type AmplifyApp struct {
	ID   string
	Name string
}

func GetAmplifyApps(ctx context.Context, cfg aws.Config, tags map[string]string) ([]AmplifyApp, error) {
	var apps []AmplifyApp

	client := amplify.NewFromConfig(cfg)
	paginator := amplify.NewListAppsPaginator(client, &amplify.ListAppsInput{})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("error listing apps: %w", err)
		}

		for _, app := range page.Apps {
			if len(tags) > 0 {
				if !matchesAllTags(app.Tags, tags) {
					continue
				}
			}

			apps = append(apps, AmplifyApp{
				ID:   *app.AppId,
				Name: *app.Name,
			})
		}
	}

	return apps, nil
}

type AmplifyDomain struct {
	AppID  string
	Domain string
}

// GetAmplifyAppDomains returns all custom domains for a given Amplify app.
func GetAmplifyAppDomains(ctx context.Context, cfg aws.Config, app AmplifyApp) ([]AmplifyDomain, error) {
	client := amplify.NewFromConfig(cfg)

	out, err := client.ListDomainAssociations(ctx, &amplify.ListDomainAssociationsInput{
		AppId: aws.String(app.ID),
	})
	if err != nil {
		return nil, fmt.Errorf("listing domain associations for app %s: %w", app.ID, err)
	}

	domains := make([]AmplifyDomain, 0, len(out.DomainAssociations))
	for _, da := range out.DomainAssociations {
		if da.DomainName == nil {
			continue
		}

		domains = append(domains, AmplifyDomain{
			AppID:  app.ID,
			Domain: *da.DomainName,
		})
	}

	return domains, nil
}
