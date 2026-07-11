package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQuotaMutationsRemainDurableWhenBatchUpdatesAreEnabled(t *testing.T) {
	truncateTables(t)
	previous := common.BatchUpdateEnabled
	common.BatchUpdateEnabled = true
	t.Cleanup(func() {
		common.BatchUpdateEnabled = previous
	})

	user := &User{Username: "quota-durability", Quota: 100}
	require.NoError(t, DB.Create(user).Error)
	token := &Token{UserId: user.Id, Name: "quota-durability", Key: "quota-durability-key", RemainQuota: 100}
	require.NoError(t, DB.Create(token).Error)

	require.NoError(t, IncreaseUserQuota(user.Id, 50, false))
	require.NoError(t, DecreaseUserQuota(user.Id, 20, false))
	require.NoError(t, IncreaseTokenQuota(token.Id, token.Key, 50))
	require.NoError(t, DecreaseTokenQuota(token.Id, token.Key, 20))

	var storedUser User
	require.NoError(t, DB.First(&storedUser, user.Id).Error)
	assert.Equal(t, 130, storedUser.Quota)
	var storedToken Token
	require.NoError(t, DB.First(&storedToken, token.Id).Error)
	assert.Equal(t, 130, storedToken.RemainQuota)
	assert.Equal(t, -30, storedToken.UsedQuota)
}
