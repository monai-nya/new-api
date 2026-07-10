package controller

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupOAuthSessionTest(t *testing.T) {
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

func makeOAuthSessionCookie(t *testing.T, user model.User, sessionVersion int64) *http.Cookie {
	t.Helper()

	store := cookie.NewStore([]byte("0123456789abcdef0123456789abcdef"))
	router := gin.New()
	router.Use(sessions.Sessions("session", store))
	router.GET("/login", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set("id", user.Id)
		session.Set("username", user.Username)
		session.Set("role", user.Role)
		session.Set("status", user.Status)
		session.Set("group", user.Group)
		session.Set("session_version", sessionVersion)
		require.NoError(t, session.Save())
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/login", nil))
	require.Equal(t, http.StatusNoContent, recorder.Code)
	require.NotEmpty(t, recorder.Result().Cookies())
	return recorder.Result().Cookies()[0]
}

func validateOAuthSessionCookie(t *testing.T, sessionCookie *http.Cookie) (*model.User, error) {
	t.Helper()

	store := cookie.NewStore([]byte("0123456789abcdef0123456789abcdef"))
	router := gin.New()
	router.Use(sessions.Sessions("session", store))

	var validatedUser *model.User
	var validationErr error
	router.GET("/validate", func(c *gin.Context) {
		validatedUser, validationErr = getOAuthBindingUser(sessions.Default(c))
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/validate", nil)
	request.AddCookie(sessionCookie)
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusNoContent, recorder.Code)
	return validatedUser, validationErr
}

func TestOAuthBindingSessionUsesCurrentSecurityState(t *testing.T) {
	setupOAuthSessionTest(t)

	user := model.User{
		Username:       "oauth-session",
		Password:       "password",
		Role:           common.RoleRootUser,
		Status:         common.UserStatusEnabled,
		Group:          "default",
		SessionVersion: 4,
	}
	require.NoError(t, model.DB.Create(&user).Error)

	t.Run("accepts current session version", func(t *testing.T) {
		sessionCookie := makeOAuthSessionCookie(t, user, user.SessionVersion)
		validatedUser, err := validateOAuthSessionCookie(t, sessionCookie)

		require.NoError(t, err)
		require.NotNil(t, validatedUser)
		assert.Equal(t, user.Id, validatedUser.Id)
		assert.Equal(t, common.RoleRootUser, validatedUser.Role)
	})

	t.Run("rejects revoked session version", func(t *testing.T) {
		sessionCookie := makeOAuthSessionCookie(t, user, user.SessionVersion-1)
		validatedUser, err := validateOAuthSessionCookie(t, sessionCookie)

		assert.Nil(t, validatedUser)
		assert.True(t, errors.Is(err, middleware.ErrInvalidSession))
	})

	t.Run("rejects disabled user", func(t *testing.T) {
		require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", user.Id).
			Update("status", common.UserStatusDisabled).Error)
		sessionCookie := makeOAuthSessionCookie(t, user, user.SessionVersion)
		validatedUser, err := validateOAuthSessionCookie(t, sessionCookie)

		assert.Nil(t, validatedUser)
		assert.True(t, errors.Is(err, middleware.ErrInvalidSession))
	})
}
