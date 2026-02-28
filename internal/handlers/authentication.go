package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/CLDWare/schoolbox-backend/config"
	contextkeys "github.com/CLDWare/schoolbox-backend/internal/contextKeys"
	models "github.com/CLDWare/schoolbox-backend/pkg/db"
	"github.com/CLDWare/schoolbox-backend/pkg/logger"
	"github.com/MonkyMars/gecho"
	"gorm.io/gorm"

	"google.golang.org/api/idtoken"
)

// RegistrationHandler handles registration-related requests
type AuthenticationHandler struct {
	quitCh chan os.Signal
	config *config.Config
	db     *gorm.DB
}

// NewRegistrationHandler creates a new registration handler
func NewAuthenticationHandler(quitCh chan os.Signal, cfg *config.Config, db *gorm.DB) *AuthenticationHandler {
	return &AuthenticationHandler{
		quitCh: quitCh,
		config: cfg,
		db:     db,
	}
}

// https://developers.google.com/identity/protocols/oauth2/native-app?utm_source=chatgpt.com#exchange-authorization-code
type GoogleOAuthTokenResponseBody struct {
	AccessToken           string `json:"access_token"`             // The token that your application sends to authorize a Google API request.
	ExpiresIn             int    `json:"expires_in"`               // The remaining lifetime of the access token in seconds.
	IdToken               string `json:"id_token"`                 // Note: This property is only returned if your request included an identity scope, such as openid, profile, or email. The value is a JSON Web Token (JWT) that contains digitally signed identity information about the user.
	RefreshToken          string `json:"refresh_token"`            // A token that you can use to obtain a new access token. Refresh tokens are valid until the user revokes access or the refresh token expires. Note that refresh tokens are always returned for installed applications.
	RefreshTokenExpiresIn int    `json:"refresh_token_expires_in"` // The remaining lifetime of the refresh token in seconds. This value is only set when the user grants time-based access.
	Scope                 string `json:"scope"`                    // The scopes of access granted by the access_token expressed as a list of space-delimited, case-sensitive strings.
	TokenType             string `json:"token_type"`               // The type of token returned. At this time, this field's value is always set to Bearer.
}

type GoogleIdTokenClaims struct {
	ATHash        string `json:"at_hash"`
	AUD           string `json:"aud"`
	AZP           string `json:"azp"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	ExpiresIn     int    `json:"exp"`
	GivenName     string `json:"given_name"`
	ISS           string `json:"iss"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
	GoogleSubject string `json:"sub"`
}

// redirects GET /login requests
// GetLogin
//
// @Summary		Login with google OAuth
// @Description	Redirect to the google OAuth endpoint
// @Tags		auth
// @Response		302
// @Router			/login [get]
func (h *AuthenticationHandler) GetLogin(w http.ResponseWriter, r *http.Request) {
	if err := gecho.Handlers.HandleMethod(w, r, http.MethodGet); err != nil {
		err.Send() // Automatically sends 405 Method Not Allowed
		return
	}

	params := url.Values{}
	params.Set("client_id", h.config.OAuth.ClientId)
	redirectURI, err := url.JoinPath("http://"+h.config.GetServerAddress(), "/api/oauth2callback")
	if err != nil {
		errMsg := fmt.Sprintf("Could not create login redirect uri: %s", err.Error())
		logger.Err(errMsg)
		gecho.InternalServerError(w).WithMessage(errMsg).Send()
	}
	params.Set("redirect_uri", redirectURI)
	params.Set("response_type", "code")
	params.Set("scope", "openid email profile")

	oauthURL := "https://accounts.google.com/o/oauth2/v2/auth?" + params.Encode()
	logger.Info(oauthURL)
	http.Redirect(w, r, oauthURL, http.StatusFound)
}

// GetOAuthCallback
//
// @Summary		Callback url for google OAuth
// @Description	The callback url for google OAuth that processes the things. Can not be used indivualy (requires google OAuth code)
// @Description Refer too google OAuth docs for more information.
// @Tags			auth
// @Response		302
// @Router			/oauth2callback [get]
func (h *AuthenticationHandler) GetOAuthCallback(w http.ResponseWriter, r *http.Request) {
	if err := gecho.Handlers.HandleMethod(w, r, http.MethodGet); err != nil {
		err.Send() // Automatically sends 405 Method Not Allowed
		return
	}

	query := r.URL.Query()
	code := query.Get("code")

	data := url.Values{}
	data.Set("code", code)
	data.Set("client_id", h.config.OAuth.ClientId)
	data.Set("client_secret", h.config.OAuth.ClientSecret)
	redirectURI, err := url.JoinPath("http://"+h.config.GetServerAddress(), "/api/oauth2callback")
	if err != nil {
		errMsg := fmt.Sprintf("Could not create login redirect uri: %s", err.Error())
		logger.Err(errMsg)
		gecho.InternalServerError(w).WithMessage(errMsg).Send()
		return
	}
	data.Set("redirect_uri", redirectURI)
	data.Set("grant_type", "authorization_code")

	resp, err := http.Post(
		"https://oauth2.googleapis.com/token",
		"application/x-www-form-urlencoded",
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		errMsg := fmt.Sprintf("Could not retrieve token using OAuth callback code: %s", err.Error())
		logger.Err(errMsg)
		gecho.InternalServerError(w).Send() // No message because we are handeling auth data and i dont want to accidently leak it or something
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 199 || 300 < resp.StatusCode {
		logger.Err(fmt.Sprintf("An %d error occured while posting to https://oauth2.googleapis.com/token", resp.StatusCode))
		gecho.InternalServerError(w).Send()

	}

	body := GoogleOAuthTokenResponseBody{}
	err = json.NewDecoder(resp.Body).Decode(&body)
	if err != nil {
		gecho.InternalServerError(w).Send() // No message because we are handeling auth data and i dont want to accidently leak it or something
		logger.Err(err.Error())
		return
	}

	ctx := context.Background()
	payload, err := idtoken.Validate(ctx, body.IdToken, h.config.OAuth.ClientId)
	if err != nil {
		gecho.InternalServerError(w).Send()
		logger.Err(err.Error())
		return
	}

	jsonClaims, err := json.Marshal(payload.Claims)
	if err != nil {
		gecho.InternalServerError(w).Send()
		logger.Err(err)
		return
	}
	var parsedClaims GoogleIdTokenClaims
	err = json.Unmarshal(jsonClaims, &parsedClaims)
	if err != nil {
		gecho.InternalServerError(w).Send()
		logger.Err(err)
		return
	}

	// Download the pfp image async (also updates from google if user already exists)
	go (func() {
		pfp_dir := "data/user_pfp/"
		err := os.Mkdir(pfp_dir, os.ModePerm)
		if err != nil && err == os.ErrExist {
			logger.Err(fmt.Errorf("failed to create directory '%s': %s", pfp_dir, err.Error()))
		}
		filename := filepath.Join(pfp_dir, fmt.Sprintf("%s.jpg", payload.Subject))
		logger.Info(filename)

		resp, err = http.Get(parsedClaims.Picture)
		if err != nil {
			logger.Err(fmt.Sprintf("Failed to download profile image from '%s': %s", parsedClaims.Picture, err.Error()))
			return
		}
		defer resp.Body.Close()

		out, err := os.Create(filename)
		if err != nil {
			logger.Err(fmt.Sprintf("Failed to create file '%s': %s", filename, err.Error()))
			return
		}
		defer out.Close()

		_, err = io.Copy(out, resp.Body)
		if err != nil {
			logger.Err(fmt.Sprintf("Failed to copy profile image from '%s' to file '%s': %s", parsedClaims.Picture, filename, err.Error()))
			return
		}
	})()

	user, err := gorm.G[models.User](h.db).Where("google_subject = ?", payload.Subject).First(ctx)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			user = models.User{
				GoogleSubject:  payload.Subject,
				ProfilePicture: parsedClaims.Picture, // TODO: deprecate because this unused
				Email:          parsedClaims.Email,
				Name:           parsedClaims.Name,
				DisplayName:    parsedClaims.GivenName,
			}
			err := gorm.G[models.User](h.db).Create(ctx, &user)
			if err != nil {
				gecho.InternalServerError(w).Send()
				err := fmt.Errorf("An error occured creating the user: %s", err.Error())
				logger.Err(err)
				return
			}
		} else {
			gecho.InternalServerError(w).Send()
			err := fmt.Errorf("An error occured retrieving the user: %s", err.Error())
			logger.Err(err)
			return
		}
	}

	// Create auth session
	session_token, err := generateSecureToken(128)
	if err != nil {
		gecho.InternalServerError(w).WithMessage("Could not create authenticated session").Send()
		logger.Err(err.Error())
		return
	}
	session := models.AuthSession{
		SessionToken: session_token,
		UserID:       user.ID,
		ExpiresAt:    time.Now().Add(h.config.OAuth.SessionDuration),
	}

	gorm.G[models.AuthSession](h.db).Create(ctx, &session)

	cookie := http.Cookie{
		Name:     "auth_session_token",
		Value:    session.SessionToken,
		Domain:   h.config.Server.Host,
		Path:     "/",
		HttpOnly: true,
		Expires:  session.ExpiresAt,
	}
	http.SetCookie(w, &cookie)
	http.Redirect(w, r, "/", http.StatusFound)
}

// GetLogout
//
// @Summary		Logout
// @Description	Invalidate the session token
// @Tags		auth requiresAuth
// @Response		302
// @Router			/logout [get]
func (h *AuthenticationHandler) GetLogout(w http.ResponseWriter, r *http.Request) {
	if err := gecho.Handlers.HandleMethod(w, r, http.MethodGet); err != nil {
		err.Send() // Automatically sends 405 Method Not Allowed
		return
	}

	ctx := r.Context()
	session, ok := ctx.Value(contextkeys.AuthSessionKey).(models.AuthSession)
	if !ok {
		gecho.InternalServerError(w).Send()
		logger.Err("Session does not exist for this user")
		return
	}

	logger.Info(session)
	gorm.G[models.AuthSession](h.db).Where("id = ?", session.ID).Update(ctx, "expires_at", time.Now())

	cookie := http.Cookie{
		Name:     "auth_session_token",
		Value:    "",
		Domain:   h.config.Server.Host,
		Path:     "/",
		HttpOnly: true,
		Expires:  time.Unix(0, 0),
	}
	http.SetCookie(w, &cookie)
	http.Redirect(w, r, "/", http.StatusFound)
}
