package middleware

import (
	"context"
	"net/http"
	"time"

	contextkeys "github.com/CLDWare/schoolbox-backend/internal/contextKeys"
	models "github.com/CLDWare/schoolbox-backend/pkg/db"
	"github.com/MonkyMars/gecho"
	"gorm.io/gorm"
)

type AuthenticationMiddleware struct {
	DB *gorm.DB
}

// AuthenticationMiddleware.Required checks if valid authentication is present and sets the contextkeys.AuthSessionKey, contextkeys.AuthUserKey values on the context (something like that)
func (mw AuthenticationMiddleware) Required(next func(w http.ResponseWriter, r *http.Request)) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		auth_session, err := r.Cookie("auth_session_token")
		if err == http.ErrNoCookie {
			gecho.Unauthorized(w).WithMessage("'auth_session_token' cookie is required for authenticated requests").Send()
			return
		} else if err != nil {
			gecho.InternalServerError(w).Send()
			return
		}
		ctx := r.Context()

		session, err := gorm.G[models.AuthSession](mw.DB).Where("session_token = ?", auth_session.Value).First(ctx)
		if err == gorm.ErrRecordNotFound {
			gecho.Unauthorized(w).WithMessage("Invalid session").Send()
			return
		} else if err != nil {
			gecho.InternalServerError(w).Send()
			return
		}

		if time.Now().After(session.ExpiresAt) {
			gecho.Unauthorized(w).WithMessage("Invalid or expired session").Send()
			return
		}

		user, err := gorm.G[models.User](mw.DB).Where("id = ?", session.UserID).First(ctx)
		if err == gorm.ErrRecordNotFound {
			gecho.Unauthorized(w).WithMessage("Invalid or expired session").Send()
			return
		} else if err != nil {
			gecho.InternalServerError(w).Send()
			return
		}

		ctx = context.WithValue(ctx, contextkeys.AuthSessionKey, session)
		ctx = context.WithValue(ctx, contextkeys.AuthUserKey, user)

		next(w, r.WithContext(ctx))
	}
}

func (mw AuthenticationMiddleware) RequiresAdmin(next func(w http.ResponseWriter, r *http.Request)) func(w http.ResponseWriter, r *http.Request) {
	return mw.Required(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		user, ok := ctx.Value(contextkeys.AuthUserKey).(models.User)
		if !ok {
			gecho.InternalServerError(w).Send()
			return
		}
		if user.Role != 1 {
			gecho.Forbidden(w).Send()
			return
		}
		next(w, r)
	})
}
