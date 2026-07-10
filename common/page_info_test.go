package common

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestGetPageQueryBoundsPagination(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name         string
		query        string
		wantPage     int
		wantPageSize int
	}{
		{name: "defaults", wantPage: 1, wantPageSize: ItemsPerPage},
		{name: "negative page", query: "?p=-1", wantPage: 1, wantPageSize: ItemsPerPage},
		{name: "negative page size", query: "?page_size=-1", wantPage: 1, wantPageSize: ItemsPerPage},
		{name: "negative legacy page size", query: "?ps=-1&size=-2", wantPage: 1, wantPageSize: ItemsPerPage},
		{name: "legacy page size", query: "?ps=25", wantPage: 1, wantPageSize: 25},
		{name: "token page size", query: "?size=30", wantPage: 1, wantPageSize: 30},
		{name: "maximum page size", query: "?page_size=101", wantPage: 1, wantPageSize: 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest("GET", "/"+tt.query, nil)

			pageInfo := GetPageQuery(ctx)

			assert.Equal(t, tt.wantPage, pageInfo.Page)
			assert.Equal(t, tt.wantPageSize, pageInfo.PageSize)
		})
	}
}

func TestPageInfoIndexesSaturateOnOverflow(t *testing.T) {
	maxInt := int(^uint(0) >> 1)
	pageInfo := PageInfo{Page: maxInt, PageSize: 100}

	assert.Equal(t, maxInt, pageInfo.GetStartIdx())
	assert.Equal(t, maxInt, pageInfo.GetEndIdx())
}
