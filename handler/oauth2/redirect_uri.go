package oauth2

// ResolveRedirectURI returns the requested URI only when it exactly matches an allowed URI.
func ResolveRedirectURI(allowedURIs string, requested string) (string, error) {
	if err := ValidateRedirectURI(allowedURIs, requested); err != nil {
		return "", err
	}
	return requested, nil
}
