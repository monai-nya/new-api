package billingexpr_test

import (
	"testing"

	"github.com/QuantumNous/new-api/pkg/billingexpr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunExprRejectsNegativeAndNonFiniteResults(t *testing.T) {
	tests := []struct {
		name   string
		expr   string
		params billingexpr.TokenParams
		want   string
	}{
		{name: "negative", expr: "p * -1", params: billingexpr.TokenParams{P: 1}, want: "non-negative"},
		{name: "positive infinity", expr: "p / c", params: billingexpr.TokenParams{P: 1}, want: "finite"},
		{name: "nan", expr: "p / c", params: billingexpr.TokenParams{}, want: "finite"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := billingexpr.RunExpr(tt.expr, tt.params)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestRunExprRejectsRequestSpecificNegativeBranch(t *testing.T) {
	expr := `param("service_tier") == "flex" ? cr * -1 : p`

	_, _, err := billingexpr.RunExprWithRequest(
		expr,
		billingexpr.TokenParams{P: 1000},
		billingexpr.RequestInput{Body: []byte(`{"service_tier":"fast"}`)},
	)
	require.NoError(t, err)

	_, _, err = billingexpr.RunExprWithRequest(
		expr,
		billingexpr.TokenParams{CR: 1000},
		billingexpr.RequestInput{Body: []byte(`{"service_tier":"flex"}`)},
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "non-negative")
}
