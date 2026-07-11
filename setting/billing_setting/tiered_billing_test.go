package billing_setting_test

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/stretchr/testify/require"
)

func TestSmokeTestExprCoversCacheTokenDimensions(t *testing.T) {
	err := billing_setting.SmokeTestExpr(`cr == 0 ? 0 : cr * -1`)
	require.Error(t, err)
}
