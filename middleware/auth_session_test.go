package middleware

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupAuthSessionTest(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}))

	oldDB := model.DB
	model.DB = db
	t.Cleanup(func() {
		model.DB = oldDB
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})
}

func makeSessionCookie(t *testing.T, user model.User, sessionRole int, sessionVersion int64) *http.Cookie {
	t.Helper()

	store := cookie.NewStore([]byte("0123456789abcdef0123456789abcdef"))
	router := gin.New()
	router.Use(sessions.Sessions("session", store))
	router.GET("/login", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set("id", user.Id)
		session.Set("username", user.Username)
		session.Set("role", sessionRole)
		session.Set("status", common.UserStatusEnabled)
		session.Set("group", user.Group)
		session.Set("session_version", sessionVersion)
		require.NoError(t, session.Save())
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/login", nil)
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusNoContent, recorder.Code)
	require.NotEmpty(t, recorder.Result().Cookies())
	return recorder.Result().Cookies()[0]
}

func TestAdminAuthUsesCurrentDatabaseRole(t *testing.T) {
	setupAuthSessionTest(t)

	user := model.User{
		Username:       "demoted-admin",
		Password:       "password",
		Role:           common.RoleCommonUser,
		Status:         common.UserStatusEnabled,
		Group:          "default",
		SessionVersion: 7,
	}
	require.NoError(t, model.DB.Create(&user).Error)
	sessionCookie := makeSessionCookie(t, user, common.RoleRootUser, user.SessionVersion)

	store := cookie.NewStore([]byte("0123456789abcdef0123456789abcdef"))
	router := gin.New()
	router.Use(sessions.Sessions("session", store))
	called := false
	router.GET("/admin", AdminAuth(), func(c *gin.Context) {
		called = true
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/admin", nil)
	request.AddCookie(sessionCookie)
	request.Header.Set("New-Api-User", strconv.Itoa(user.Id))
	router.ServeHTTP(recorder, request)

	assert.False(t, called)
	assert.Equal(t, http.StatusOK, recorder.Code)
}

func TestUserAuthRejectsRevokedSessionVersion(t *testing.T) {
	setupAuthSessionTest(t)

	user := model.User{
		Username:       "revoked-session",
		Password:       "password",
		Role:           common.RoleCommonUser,
		Status:         common.UserStatusEnabled,
		Group:          "default",
		SessionVersion: 3,
	}
	require.NoError(t, model.DB.Create(&user).Error)
	sessionCookie := makeSessionCookie(t, user, user.Role, user.SessionVersion-1)

	store := cookie.NewStore([]byte("0123456789abcdef0123456789abcdef"))
	router := gin.New()
	router.Use(sessions.Sessions("session", store))
	called := false
	router.GET("/self", UserAuth(), func(c *gin.Context) {
		called = true
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/self", nil)
	request.AddCookie(sessionCookie)
	request.Header.Set("New-Api-User", strconv.Itoa(user.Id))
	router.ServeHTTP(recorder, request)

	assert.False(t, called)
	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
}
