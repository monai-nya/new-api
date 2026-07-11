package service

import (
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type billingSafetyFunding struct {
	settleDelta int
}

func (f *billingSafetyFunding) Source() string       { return BillingSourceWallet }
func (f *billingSafetyFunding) PreConsume(int) error { return nil }
func (f *billingSafetyFunding) Refund() error        { return nil }
func (f *billingSafetyFunding) Settle(delta int) error {
	f.settleDelta = delta
	return nil
}

func TestBillingSessionRejectsNegativeActualQuota(t *testing.T) {
	funding := &billingSafetyFunding{}
	session := &BillingSession{
		relayInfo: &relaycommon.RelayInfo{IsPlayground: true},
		funding:   funding,
	}

	err := session.Settle(-500)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "non-negative")
	assert.Zero(t, funding.settleDelta)
	fundingQuota, tokenQuota := session.GetChargedQuotas()
	assert.Zero(t, fundingQuota)
	assert.Zero(t, tokenQuota)
}
