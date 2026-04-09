package oauth2

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	gooauth2 "github.com/go-oauth2/oauth2/v4"
	oauth2errors "github.com/go-oauth2/oauth2/v4/errors"
	"github.com/go-oauth2/oauth2/v4/manage"
	oauth2server "github.com/go-oauth2/oauth2/v4/server"
	"github.com/go-oauth2/oauth2/v4/store"
	oredis "github.com/go-oauth2/redis/v4"
	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"

	"sso-server/conf"
	"sso-server/dal/db"
)

type OAuth2 struct {
	cfg     *conf.Config
	db      *gorm.DB
	manager *manage.Manager
	server  *oauth2server.Server
}

func New(cfg *conf.Config) (*OAuth2, error) {
	tokenStore := oredis.NewRedisStore(toRedisV8Options(cfg))
	return NewWithStores(cfg, db.DB, tokenStore)
}

func NewWithStores(cfg *conf.Config, database *gorm.DB, tokenStore gooauth2.TokenStore) (*OAuth2, error) {
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

	manager := manage.NewDefaultManager()
	manager.SetAuthorizeCodeExp(5 * time.Minute)
	manager.SetAuthorizeCodeTokenCfg(&manage.Config{
		AccessTokenExp:    cfg.Security.AccessTokenExpire,
		RefreshTokenExp:   0,
		IsGenerateRefresh: false,
	})
	manager.SetPasswordTokenCfg(&manage.Config{
		AccessTokenExp:    cfg.Security.AccessTokenExpire,
		RefreshTokenExp:   0,
		IsGenerateRefresh: false,
	})
	manager.MapTokenStorage(tokenStore)
	manager.MapClientStorage(NewClientStore(database))
	manager.SetValidateURIHandler(ValidateRedirectURI)

	srv := oauth2server.NewDefaultServer(manager)
	srv.SetAllowedResponseType(gooauth2.Code)
	srv.SetAllowedGrantType(gooauth2.AuthorizationCode)
	srv.SetClientInfoHandler(clientInfoHandler)
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
		return cfg.Security.AccessTokenExpire, nil
	})
	srv.SetTokenType("Bearer")

	return &OAuth2{
		cfg:     cfg,
		db:      database,
		manager: manager,
		server:  srv,
	}, nil
}

func (o *OAuth2) IssueTokenForUser(ctx context.Context, r *http.Request, userID string) (map[string]interface{}, error) {
	if r == nil {
		return nil, oauth2errors.ErrServerError
	}

	tgr := &gooauth2.TokenGenerateRequest{
		ClientID:     "api",
		ClientSecret: "",
		UserID:       userID,
		Request:      r,
	}
	ti, err := o.manager.GenerateAccessToken(ctx, gooauth2.PasswordCredentials, tgr)
	if err != nil {
		return nil, err
	}
	return o.server.GetTokenData(ti), nil
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

	finalRedirectURI, err := ResolveRedirectURI(client.GetDomain(), req.RedirectURI)
	if err != nil {
		o.redirectOrWriteAuthorizeError(c, req, err)
		return
	}
	req.RedirectURI = finalRedirectURI

	userID, err := o.server.UserAuthorizationHandler(c.Writer, c.Request)
	if err != nil {
		o.redirectOrWriteAuthorizeError(c, req, err)
		return
	}
	req.UserID = userID

	ti, err := o.server.GetAuthorizeToken(ctx, req)
	if err != nil {
		o.redirectOrWriteAuthorizeError(c, req, err)
		return
	}

	data := o.server.GetAuthorizeData(req.ResponseType, ti)
	redirectTo, err := o.server.GetRedirectURI(req, data)
	if err != nil {
		o.writeTokenError(c, oauth2errors.ErrServerError)
		return
	}
	c.Redirect(http.StatusFound, redirectTo)
}

func (o *OAuth2) HandleToken(c *gin.Context) {
	if err := o.server.HandleTokenRequest(c.Writer, c.Request); err != nil {
		o.writeTokenError(c, err)
	}
}

// ValidateToken validates a bearer token and returns token info
// This is used by the OAuth handler to get user info
func (o *OAuth2) ValidateToken(r *http.Request) (gooauth2.TokenInfo, error) {
	return o.server.ValidationBearerToken(r)
}

func (o *OAuth2) redirectOrWriteAuthorizeError(c *gin.Context, req *oauth2server.AuthorizeRequest, err error) {
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
	data, status, header := o.server.GetErrorData(err)
	for k, vals := range header {
		for _, v := range vals {
			c.Writer.Header().Add(k, v)
		}
	}
	c.JSON(status, data)
}

func clientInfoHandler(r *http.Request) (string, string, error) {
	clientID, clientSecret, err := oauth2server.ClientBasicHandler(r)
	if err == nil && clientID != "" {
		return clientID, clientSecret, nil
	}
	return oauth2server.ClientFormHandler(r)
}

func toRedisV8Options(cfg *conf.Config) *redis.Options {
	raw := cfg.Cache.URL
	opt, err := redis.ParseURL(raw)
	if err == nil {
		if cfg.Cache.Password != "" {
			opt.Password = cfg.Cache.Password
		}
		return opt
	}

	opt = &redis.Options{Addr: raw}
	if cfg.Cache.Password != "" {
		opt.Password = cfg.Cache.Password
	}
	return opt
}
