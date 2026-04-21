package api

import (
	"encoding/gob"
	"net/http"

	"github.com/alireza0/s-ui/database/model"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

const (
	loginUser = "LOGIN_USER"
)

// secureCookie indicates whether the cookie should have the Secure flag set.
// It is set to true when TLS is configured.
var secureCookie bool

func init() {
	gob.Register(model.User{})
}

// SetSecureCookie configures whether session cookies should have the Secure flag.
func SetSecureCookie(secure bool) {
	secureCookie = secure
}

// newSessionOptions creates a sessions.Options with security attributes applied.
func newSessionOptions(maxAge int) sessions.Options {
	return sessions.Options{
		Path:     "/",
		MaxAge:   maxAge,
		Secure:   secureCookie,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	}
}

func SetLoginUser(c *gin.Context, userName string, maxAge int) error {
	age := 0
	if maxAge > 0 {
		age = maxAge * 60
	}

	s := sessions.Default(c)
	s.Set(loginUser, userName)
	s.Options(newSessionOptions(age))

	return s.Save()
}

func SetMaxAge(c *gin.Context) error {
	s := sessions.Default(c)
	s.Options(newSessionOptions(0))
	return s.Save()
}

func GetLoginUser(c *gin.Context) string {
	s := sessions.Default(c)
	obj := s.Get(loginUser)
	if obj == nil {
		return ""
	}
	objStr, ok := obj.(string)
	if !ok {
		return ""
	}
	return objStr
}

func IsLogin(c *gin.Context) bool {
	return GetLoginUser(c) != ""
}

func ClearSession(c *gin.Context) {
	s := sessions.Default(c)
	s.Clear()
	s.Options(newSessionOptions(-1))
	s.Save()
}
