package oauth2

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	gooauth2 "github.com/go-oauth2/oauth2/v4"
	oauth2errors "github.com/go-oauth2/oauth2/v4/errors"
	"github.com/go-oauth2/oauth2/v4/manage"
	"github.com/go-oauth2/oauth2/v4/models"
	oauth2server "github.com/go-oauth2/oauth2/v4/server"
	"github.com/go-oauth2/oauth2/v4/store"
	"gorm.io/gorm"

	"sso-server/conf"
	"sso-server/dal/db"
	"sso-server/dal/kv"
	"sso-server/handler/audit"
	"sso-server/manager/oauth"
)

type OAuth2 struct {
	cfg     *conf.Config
	db      *gorm.DB
	manager *manage.Manager
	server  *oauth2server.Server
	clients *db.OAuthClientRepository
	signer  *tokenSigner
	refresh oauth.RefreshStore
}

type authorizationSessionKey struct{}

func New(cfg *conf.Config) (*OAuth2, error) {
	if kv.Client == nil {
		return nil, errors.New("Redis is required for OAuth refresh tokens")
	}
	tokenStore := oauth.NewAuthorizationCodeStore(kv.NewNamespacedStore(kv.NewRedisStore(kv.Client), conf.GetEnvironmentName()))
	return newWithStores(cfg, db.DB, tokenStore, oauth.NewRedisRefreshStore(kv.Client))
}

func NewWithStores(cfg *conf.Config, database *gorm.DB, tokenStore gooauth2.TokenStore) (*OAuth2, error) {
	return newWithStores(cfg, database, tokenStore, oauth.NewMemoryRefreshStore())
}

func newWithStores(cfg *conf.Config, database *gorm.DB, tokenStore gooauth2.TokenStore, refresh oauth.RefreshStore) (*OAuth2, error) {
	if cfg == nil {
		return nil, oauth2errors.ErrServerError
	}

	if database == nil {
		return nil, oauth2errors.ErrServerError
	}

	if tokenStore == nil {
		s, err := store.NewMemoryTokenStore()
		if err != nil {
			return nil, err
		}
		tokenStore = s
	}
	signer, err := newTokenSigner(cfg.Tokens.OAuth)
	if err != nil {
		return nil, err
	}
	manager := manage.NewDefaultManager()
	clients := db.NewOAuthClientRepository(database)
	owner := &OAuth2{cfg: cfg, db: database, manager: manager, clients: clients, signer: signer, refresh: refresh}
	manager.MapAccessGenerate(jwtAccessGenerator{owner: owner})
	manager.SetExtractExtensionHandler(func(request *gooauth2.TokenGenerateRequest, info gooauth2.ExtendableTokenInfo) {
		if sessionID, ok := request.Request.Context().Value(authorizationSessionKey{}).(string); ok && sessionID != "" {
			extension := info.GetExtension()
			extension.Set("session_id", sessionID)
			extension.Set("authorized_at", strconv.FormatInt(time.Now().UnixMicro(), 10))
			info.SetExtension(extension)
		}
	})
	manager.SetAuthorizeCodeExp(cfg.Tokens.OAuth.AuthorizationCodeTTL)
	manager.SetAuthorizeCodeTokenCfg(&manage.Config{
		AccessTokenExp:    cfg.Tokens.OAuth.AccessTokenTTL,
		RefreshTokenExp:   0,
		IsGenerateRefresh: false,
	})
	manager.MapTokenStorage(tokenStore)
	clientStore := NewClientStore(database)
	manager.MapClientStorage(clientStore)
	manager.SetValidateURIHandler(ValidateRedirectURI)

	srv := oauth2server.NewDefaultServer(manager)
	srv.Config.AllowedCodeChallengeMethods = []gooauth2.CodeChallengeMethod{
		gooauth2.CodeChallengePlain,
		gooauth2.CodeChallengeS256,
	}
	srv.SetAllowedResponseType(gooauth2.Code)
	srv.SetAllowedGrantType(gooauth2.AuthorizationCode)
	srv.SetClientInfoHandler(clientInfoHandler(clientStore))
	srv.SetUserAuthorizationHandler(func(w http.ResponseWriter, r *http.Request) (string, error) {
		if r == nil {
			return "", oauth2errors.ErrAccessDenied
		}

		userID, ok := r.Context().Value("user_id").(string)
		if !ok || userID == "" {
			return "", oauth2errors.ErrAccessDenied
		}
		return userID, nil
	})
	srv.SetAccessTokenExpHandler(func(w http.ResponseWriter, r *http.Request) (time.Duration, error) {
		return cfg.Tokens.OAuth.AccessTokenTTL, nil
	})
	srv.SetTokenType("Bearer")

	owner.server = srv
	return owner, nil
}

func (o *OAuth2) IssueTokenForUser(ctx context.Context, r *http.Request, userID string) (map[string]interface{}, error) {
	return nil, oauth2errors.ErrUnsupportedGrantType
}

func (o *OAuth2) HandleAuthorize(c *gin.Context) {
	ctx := c.Request.Context()
	req, err := o.server.ValidationAuthorizeRequest(c.Request)
	if err != nil {
		o.writeTokenError(c, err)
		return
	}

	client, err := o.manager.GetClient(ctx, req.ClientID)
	if err != nil {
		o.redirectOrWriteAuthorizeError(c, req, oauth2errors.ErrInvalidClient)
		return
	}

	audit.Client(c, req.ClientID)
	audit.Target(c, "oauth_client", req.ClientID)
	finalRedirectURI, err := ResolveRedirectURI(client.GetDomain(), req.RedirectURI)
	if err != nil {
		req.RedirectURI = ""
		o.redirectOrWriteAuthorizeError(c, req, err)
		return
	}
	req.RedirectURI = finalRedirectURI
	configured, err := o.clients.FindByClientID(ctx, req.ClientID)
	if err != nil || len(configured.Audiences) == 0 {
		o.redirectOrWriteAuthorizeError(c, req, oauth2errors.ErrInvalidScope)
		return
	}
	requestedScopes := strings.Fields(req.Scope)
	for _, scope := range requestedScopes {
		if !slices.Contains(configured.AllowedScopes, scope) {
			o.redirectOrWriteAuthorizeError(c, req, oauth2errors.ErrInvalidScope)
			return
		}
	}
	req.Scope = strings.Join(requestedScopes, " ")
	if err := (AuthorizationCodePolicy{}).ValidateAuthorizationRequest(c.Request, client.IsPublic()); err != nil {
		o.redirectOrWriteAuthorizeError(c, req, err)
		return
	}

	userID, err := o.server.UserAuthorizationHandler(c.Writer, c.Request)
	if err != nil {
		o.redirectOrWriteAuthorizeError(c, req, err)
		return
	}
	req.UserID = userID
	c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), authorizationSessionKey{}, c.GetString("session_id")))
	req.Request = c.Request

	ti, err := o.server.GetAuthorizeToken(ctx, req)
	if err != nil {
		o.redirectOrWriteAuthorizeError(c, req, err)
		return
	}

	if err := db.NewUserOAuthClientRepository(o.db).RecordLogin(ctx, req.UserID, req.ClientID, time.Now()); err != nil {
		o.writeTokenError(c, oauth2errors.ErrServerError)
		return
	}

	data := o.server.GetAuthorizeData(req.ResponseType, ti)
	redirectTo, err := o.server.GetRedirectURI(req, data)
	if err != nil {
		o.writeTokenError(c, oauth2errors.ErrServerError)
		return
	}
	audit.Completed(c, "authorization_code_issued")
	audit.Success(c)
	c.Redirect(http.StatusFound, redirectTo)
}

func (o *OAuth2) HandleToken(c *gin.Context) {
	grantType, request, err := o.server.ValidationTokenRequest(c.Request)
	if err != nil {
		o.writeTokenError(c, err)
		return
	}
	if grantType == gooauth2.Refreshing {
		o.handleRefresh(c, request)
		return
	}
	if grantType != gooauth2.AuthorizationCode {
		o.writeTokenError(c, oauth2errors.ErrUnauthorizedClient)
		return
	}
	info, err := o.server.GetAccessToken(c.Request.Context(), grantType, request)
	if err != nil {
		o.writeTokenError(c, err)
		return
	}
	client, err := o.clients.FindByClientID(c.Request.Context(), info.GetClientID())
	if err != nil || len(client.Audiences) == 0 {
		o.writeTokenError(c, oauth2errors.ErrInvalidScope)
		return
	}
	refreshToken, err := oauth.NewRefreshToken()
	if err != nil {
		o.writeTokenError(c, err)
		return
	}
	extended, ok := info.(gooauth2.ExtendableTokenInfo)
	if !ok {
		o.writeTokenError(c, oauth2errors.ErrInvalidGrant)
		return
	}
	authorizedAt, err := strconv.ParseInt(extended.GetExtension().Get("authorized_at"), 10, 64)
	if err != nil {
		o.writeTokenError(c, oauth2errors.ErrInvalidGrant)
		return
	}
	grant := oauth.RefreshGrant{UserID: info.GetUserID(), ClientID: info.GetClientID(), Audiences: client.Audiences, Scopes: strings.Fields(info.GetScope()), AuthorizedAt: authorizedAt, ExpiresAt: time.Now().Add(o.cfg.Tokens.OAuth.RefreshTokenTTL)}
	if err := o.refresh.Issue(c.Request.Context(), refreshToken, grant); err != nil {
		if errors.Is(err, oauth.ErrInvalidRefresh) {
			o.writeTokenError(c, oauth2errors.ErrInvalidGrant)
		} else {
			o.writeTokenError(c, err)
		}
		return
	}
	data := o.server.GetTokenData(info)
	data["refresh_token"] = refreshToken
	writeTokenResponse(c, data)
}

// ValidateToken validates a bearer token and returns token info
// This is used by the OAuth handler to get user info
func (o *OAuth2) ValidateToken(r *http.Request) (gooauth2.TokenInfo, error) {
	raw, ok := o.server.AccessTokenResolveHandler(r)
	if !ok {
		return nil, oauth2errors.ErrInvalidAccessToken
	}
	claims, err := o.signer.parse(raw)
	if err != nil {
		return nil, oauth2errors.ErrInvalidAccessToken
	}
	info := models.NewToken()
	info.SetUserID(claims.Subject)
	info.SetClientID(claims.ClientID)
	info.SetScope(claims.Scope)
	return info, nil
}

func (o *OAuth2) handleRefresh(c *gin.Context, request *gooauth2.TokenGenerateRequest) {
	ctx := c.Request.Context()
	grant, err := o.refresh.Load(ctx, request.Refresh)
	if err != nil {
		o.writeTokenError(c, oauth2errors.ErrInvalidGrant)
		return
	}
	client, err := o.clients.FindByClientID(ctx, grant.ClientID)
	if err != nil || request.ClientID != grant.ClientID || !client.ClientType.IsValid() || (client.ClientType.IsPublic() && request.ClientSecret != "") || (!client.ClientType.IsPublic() && subtle.ConstantTimeCompare([]byte(client.ClientSecret), []byte(request.ClientSecret)) != 1) {
		o.writeTokenError(c, oauth2errors.ErrInvalidClient)
		return
	}
	user, err := db.NewUserRepository(o.db).FindByID(ctx, grant.UserID)
	if err != nil || !user.IsActive {
		o.writeTokenError(c, oauth2errors.ErrInvalidGrant)
		return
	}
	allowedScopes := make([]string, 0, len(grant.Scopes))
	for _, scope := range grant.Scopes {
		if slices.Contains(client.AllowedScopes, scope) {
			allowedScopes = append(allowedScopes, scope)
		}
	}
	if request.Scope != "" {
		selected := strings.Fields(request.Scope)
		for _, scope := range selected {
			if !slices.Contains(allowedScopes, scope) {
				o.writeTokenError(c, oauth2errors.ErrInvalidScope)
				return
			}
		}
		allowedScopes = selected
	}
	audiences := make([]string, 0, len(grant.Audiences))
	for _, audience := range grant.Audiences {
		if slices.Contains(client.Audiences, audience) {
			audiences = append(audiences, audience)
		}
	}
	if len(audiences) == 0 {
		o.writeTokenError(c, oauth2errors.ErrInvalidGrant)
		return
	}
	access, err := o.signer.sign(grant.UserID, grant.ClientID, audiences, allowedScopes, time.Now())
	if err != nil {
		o.writeTokenError(c, err)
		return
	}
	newRefresh, err := oauth.NewRefreshToken()
	if err != nil {
		o.writeTokenError(c, err)
		return
	}
	grant.Audiences, grant.Scopes = audiences, allowedScopes
	if err := o.refresh.Rotate(ctx, request.Refresh, newRefresh, grant); err != nil {
		if errors.Is(err, oauth.ErrInvalidRefresh) {
			o.writeTokenError(c, oauth2errors.ErrInvalidGrant)
		} else {
			o.writeTokenError(c, err)
		}
		return
	}
	data := map[string]interface{}{"access_token": access, "token_type": "Bearer", "expires_in": int64(o.signer.ttl / time.Second), "refresh_token": newRefresh}
	if len(allowedScopes) > 0 {
		data["scope"] = strings.Join(allowedScopes, " ")
	}
	writeTokenResponse(c, data)
}

func (o *OAuth2) RevokeUserRefreshTokens(ctx context.Context, userID string) error {
	return o.refresh.RevokeUser(ctx, userID)
}

func writeTokenResponse(c *gin.Context, data map[string]interface{}) {
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	c.JSON(http.StatusOK, data)
}

func (o *OAuth2) redirectOrWriteAuthorizeError(c *gin.Context, req *oauth2server.AuthorizeRequest, err error) {
	audit.Failure(c, "OAUTH_AUTHORIZATION_FAILED")
	data, _, _ := o.server.GetErrorData(err)
	if req != nil && req.RedirectURI != "" {
		redirectTo, e := o.server.GetRedirectURI(req, data)
		if e == nil {
			c.Redirect(http.StatusFound, redirectTo)
			return
		}
	}
	o.writeTokenError(c, err)
}

func (o *OAuth2) writeTokenError(c *gin.Context, err error) {
	audit.Failure(c, "OAUTH_AUTHORIZATION_FAILED")
	data, status, header := o.server.GetErrorData(err)
	for k, vals := range header {
		for _, v := range vals {
			c.Writer.Header().Add(k, v)
		}
	}
	c.JSON(status, data)
}

func clientInfoHandler(clientStore *ClientStore) oauth2server.ClientInfoHandler {
	return func(r *http.Request) (string, string, error) {
		clientID, clientSecret, err := oauth2server.ClientBasicHandler(r)
		if err != nil || clientID == "" {
			clientID, clientSecret, err = oauth2server.ClientFormHandler(r)
			if err != nil {
				return "", "", err
			}
		}
		client, err := clientStore.GetByID(r.Context(), clientID)
		if err != nil {
			return "", "", err
		}
		if client.IsPublic() && clientSecret != "" {
			return "", "", oauth2errors.ErrInvalidClient
		}
		return clientID, clientSecret, nil
	}
}
