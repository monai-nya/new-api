package service

import (
	"context"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRefundTaskQuotaUsesActuallyChargedAmounts(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 30, 30, 30
	const initQuota, tokenRemain = 10_000, 5_000
	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-recorded-charge", tokenRemain)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, 3_000, tokenID, BillingSourceWallet, 0)
	task.PrivateData.FundingChargedQuota = common.GetPointer(1_000)
	task.PrivateData.TokenChargedQuota = common.GetPointer(400)

	RefundTaskQuota(ctx, task, "settlement was only partially charged")

	assert.Equal(t, initQuota+1_000, getUserQuota(t, userID))
	assert.Equal(t, tokenRemain+400, getTokenRemainQuota(t, tokenID))
	log := getLastLog(t)
	require.NotNil(t, log)
	assert.Equal(t, 1_000, log.Quota)
}

func TestApplySunoTaskResponseCASPreventsDuplicateRefund(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 31, 31, 31
	const initQuota, preConsumed, tokenRemain = 10_000, 3_000, 5_000
	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-suno-cas", tokenRemain)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)
	task.Status = model.TaskStatusInProgress
	require.NoError(t, model.DB.Create(task).Error)

	var firstPoll model.Task
	var secondPoll model.Task
	require.NoError(t, model.DB.First(&firstPoll, task.ID).Error)
	require.NoError(t, model.DB.First(&secondPoll, task.ID).Error)
	response := dto.SunoDataResponse{
		TaskID:     task.TaskID,
		Status:     string(model.TaskStatusFailure),
		FailReason: "upstream failure",
	}

	require.NoError(t, applySunoTaskResponse(ctx, &firstPoll, response))
	require.NoError(t, applySunoTaskResponse(ctx, &secondPoll, response))

	assert.Equal(t, initQuota+preConsumed, getUserQuota(t, userID))
	assert.Equal(t, tokenRemain+preConsumed, getTokenRemainQuota(t, tokenID))
	assert.Equal(t, int64(1), countLogs(t))
}
