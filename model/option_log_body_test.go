package model

import (
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateOptionValueLogBodyLimits(t *testing.T) {
	for _, key := range []string{"LogRequestBodyMaxKB", "LogResponseBodyMaxKB"} {
		t.Run(key, func(t *testing.T) {
			require.NoError(t, validateOptionValue(key, "1"))
			require.NoError(t, validateOptionValue(key, strconv.Itoa(common.MaxLogBodySizeKB)))
			assert.Error(t, validateOptionValue(key, "0"))
			assert.Error(t, validateOptionValue(key, strconv.Itoa(common.MaxLogBodySizeKB+1)))
			assert.Error(t, validateOptionValue(key, "not-a-number"))
		})
	}
}
