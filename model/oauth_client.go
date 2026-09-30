package model

// OAuthClientType describes how an OAuth client authenticates at the token endpoint.
type OAuthClientType string

const (
	// OAuthClientTypeConfidential identifies a server-side client with a secret.
	OAuthClientTypeConfidential OAuthClientType = "confidential"
	// OAuthClientTypePublic identifies a native or browser client without a secret.
	OAuthClientTypePublic OAuthClientType = "public"
)

// IsValid reports whether the client type is supported.
func (t OAuthClientType) IsValid() bool {
	return t == OAuthClientTypeConfidential || t == OAuthClientTypePublic
}

// IsPublic reports whether the client is a public OAuth client.
func (t OAuthClientType) IsPublic() bool {
	return t == OAuthClientTypePublic
}

type OAuthClient struct {
	ID            uint            `gorm:"primaryKey;autoIncrement"`
	Name          string          `gorm:"type:varchar(50);not null"`
	ClientID      string          `gorm:"type:varchar(50);uniqueIndex;not null"`
	ClientSecret  string          `gorm:"type:varchar(255);not null"`
	ClientType    OAuthClientType `gorm:"type:varchar(20);not null;default:confidential"`
	Audiences     []string        `gorm:"type:jsonb;serializer:json;not null;default:'[]'"`
	AllowedScopes []string        `gorm:"type:jsonb;serializer:json;not null;default:'[]'"`
	HomepageURL   string          `gorm:"type:text;not null"`
	RedirectURI   string          `gorm:"type:text;not null"`
	LogoutURI     string          `gorm:"type:text"`
	LogoURL       *string         `gorm:"type:text"`
	LogoObjectKey *string         `gorm:"type:varchar(512)"`
}

func (OAuthClient) TableName() string {
	return "oauth_clients"
}
