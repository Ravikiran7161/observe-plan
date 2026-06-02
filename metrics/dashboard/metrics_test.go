package dashboard

import (
	"testing"

	"github.com/thermofisher/cmd-saas/observe-plane/metrics/dashboard/resource"
)

func TestCognitoSignInMetricsAggregatesAllPoolClients(t *testing.T) {
	t.Parallel()

	props := CognitoSignInMetrics("us-east-1", []resource.CognitoUserPool{
		{
			ID:        "pool-1",
			ClientIDs: []string{"client-a", "client-b"},
		},
	})

	if len(props.Metrics) != 3 {
		t.Fatalf("expected 3 metric entries, got %d", len(props.Metrics))
	}

	firstHiddenOptions, ok := props.Metrics[0][6].(map[string]any)
	if !ok {
		t.Fatalf("expected first hidden metric options map, got %T", props.Metrics[0][6])
	}
	if firstHiddenOptions["id"] != "m0_0" {
		t.Fatalf("expected first hidden metric id m0_0, got %v", firstHiddenOptions["id"])
	}
	if firstHiddenOptions["visible"] != false {
		t.Fatalf("expected first hidden metric to be hidden, got %v", firstHiddenOptions["visible"])
	}

	secondHiddenOptions, ok := props.Metrics[1][6].(map[string]any)
	if !ok {
		t.Fatalf("expected second hidden metric options map, got %T", props.Metrics[1][6])
	}
	if secondHiddenOptions["id"] != "m0_1" {
		t.Fatalf("expected second hidden metric id m0_1, got %v", secondHiddenOptions["id"])
	}

	expressionMetric, ok := props.Metrics[2][0].(map[string]any)
	if !ok {
		t.Fatalf("expected expression metric map, got %T", props.Metrics[2][0])
	}
	if expressionMetric["expression"] != "m0_0 + m0_1" {
		t.Fatalf("expected aggregation expression over both clients, got %v", expressionMetric["expression"])
	}
	if expressionMetric["label"] != "pool-1" {
		t.Fatalf("expected pool label pool-1, got %v", expressionMetric["label"])
	}
}

func TestCognitoSignInMetricsSkipsPoolsWithoutClients(t *testing.T) {
	t.Parallel()

	props := CognitoSignInMetrics("us-east-1", []resource.CognitoUserPool{
		{
			ID:        "pool-empty",
			ClientIDs: nil,
		},
	})

	if len(props.Metrics) != 0 {
		t.Fatalf("expected no metrics for pools without clients, got %d", len(props.Metrics))
	}
}
