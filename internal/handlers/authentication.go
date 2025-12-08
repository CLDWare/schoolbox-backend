package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/CLDWare/schoolbox-backend/config"
	models "github.com/CLDWare/schoolbox-backend/pkg/db"
	"github.com/CLDWare/schoolbox-backend/pkg/logger"
	"github.com/MonkyMars/gecho"
	"gorm.io/gorm"

	"google.golang.org/api/idtoken"
)

// RegistrationHandler handles registration-related requests
type AuthenticationHandler struct {
	config           *config.Config
	db               *gorm.DB
	websocketHandler *WebsocketHandler
}

// NewRegistrationHandler creates a new registration handler
func NewAuthenticationHandler(cfg *config.Config, db *gorm.DB) *AuthenticationHandler {
	return &AuthenticationHandler{
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
func (h *AuthenticationHandler) GetLogin(w http.ResponseWriter, r *http.Request) {
	if err := gecho.Handlers.HandleMethod(w, r, http.MethodGet); err != nil {
		err.Send() // Automatically sends 405 Method Not Allowed
		return
	}

	params := url.Values{}
	params.Set("client_id", h.config.OAuth.ClientId)
	redirectURI, err := url.JoinPath("http://"+h.config.GetServerAddress(), "/oauth2callback")
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
	redirectURI, err := url.JoinPath("http://"+h.config.GetServerAddress(), "/oauth2callback")
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

	user, err := gorm.G[models.User](h.db).Where("google_subject = ?", payload.Subject).First(ctx)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			user := models.User{
				GoogleSubject: payload.Subject,
				Email:         parsedClaims.Email,
				Name:          parsedClaims.Name,
				DisplayName:   parsedClaims.GivenName,
			}
			gorm.G[models.User](h.db).Create(ctx, &user)
		} else {
			gecho.InternalServerError(w).Send()
			err := fmt.Errorf("An error occured retrieving/creating the user: %s", err.Error())
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
	}
	http.SetCookie(w, &cookie)
	http.Redirect(w, r, "/me", http.StatusFound)
}
