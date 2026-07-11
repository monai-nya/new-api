package model

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestRefundSubscriptionPreConsumeIsAtomicAndIdempotent(t *testing.T) {
	truncateTables(t)

	sub := &UserSubscription{
		UserId:      1,
		AmountTotal: 10_000,
		AmountUsed:  1_100,
		Status:      "active",
	}
	require.NoError(t, DB.Create(sub).Error)
	record := &SubscriptionPreConsumeRecord{
		RequestId:          "refund-atomic-request",
		UserId:             sub.UserId,
		UserSubscriptionId: sub.Id,
		PreConsumed:        100,
		Status:             "consumed",
	}
	require.NoError(t, DB.Create(record).Error)

	const callbackName = "test:fail_subscription_refund_record_update"
	require.NoError(t, DB.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Name == "SubscriptionPreConsumeRecord" {
			tx.AddError(errors.New("forced refund record update failure"))
		}
	}))
	callbackRegistered := true
	t.Cleanup(func() {
		if callbackRegistered {
			_ = DB.Callback().Update().Remove(callbackName)
		}
	})

	err := RefundSubscriptionPreConsume(record.RequestId)
	require.Error(t, err)

	var afterFailure UserSubscription
	require.NoError(t, DB.First(&afterFailure, sub.Id).Error)
	assert.Equal(t, int64(1_100), afterFailure.AmountUsed)
	var recordAfterFailure SubscriptionPreConsumeRecord
	require.NoError(t, DB.First(&recordAfterFailure, record.Id).Error)
	assert.Equal(t, "consumed", recordAfterFailure.Status)

	require.NoError(t, DB.Callback().Update().Remove(callbackName))
	callbackRegistered = false
	require.NoError(t, RefundSubscriptionPreConsume(record.RequestId))
	require.NoError(t, RefundSubscriptionPreConsume(record.RequestId))

	var refunded UserSubscription
	require.NoError(t, DB.First(&refunded, sub.Id).Error)
	assert.Equal(t, int64(1_000), refunded.AmountUsed)
	var refundedRecord SubscriptionPreConsumeRecord
	require.NoError(t, DB.First(&refundedRecord, record.Id).Error)
	assert.Equal(t, "refunded", refundedRecord.Status)
}
