package core

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"
)

const (
	AuthBasic  = "basic"
	AuthBearer = "bearer"
	AuthAPIKey = "api-key"
)

// Empty mode preserves profiles created before explicit authentication modes.
func AuthMode(mode string) string {
	if mode == "" {
		return AuthBasic
	}
	return mode
}
func APIKeyHeader(header string) string {
	if header == "" {
		return http.CanonicalHeaderKey("X-API-Key")
	}
	return http.CanonicalHeaderKey(header)
}
func validateAuthConfig(mode, header string) error {
	switch AuthMode(mode) {
	case AuthBasic, AuthBearer:
		if header != "" {
			return errors.New("API key header is only valid for API key authentication")
		}
	case AuthAPIKey:
		// A small allowlist avoids a general-purpose header editor and routing headers.
		switch APIKeyHeader(header) {
		case "X-Api-Key", "Api-Key", "X-Auth-Token", "X-API-Key":
		default:
			return errors.New("Choose X-API-Key, Api-Key or X-Auth-Token for the API key header")
		}
	default:
		return errors.New("Unsupported authentication method")
	}
	return nil
}

var bearerValue = regexp.MustCompile(`^[A-Za-z0-9._~+/\-]+=*$`)

func validateCredentialValue(c Credentials) error {
	if err := validateAuthConfig(c.AuthType, c.AuthHeader); err != nil {
		return err
	}
	switch AuthMode(c.AuthType) {
	case AuthBasic:
		if c.Login == "" || c.Password == "" || strings.Contains(c.Login, ":") {
			return errors.New("Enter login and password; login cannot contain a colon")
		}
		if c.Token != "" {
			return errors.New("Token is not valid for login and password authentication")
		}
	case AuthBearer, AuthAPIKey:
		if c.Login != "" || c.Password != "" {
			return errors.New("Login and password are not valid for token authentication")
		}
		if c.Token == "" || len(c.Token) > 16384 {
			return errors.New("Enter a token or API key of at most 16 KB")
		}
		for _, ch := range c.Token {
			if ch < 33 || ch > 126 {
				return errors.New("Token or API key cannot contain spaces, line breaks or non-ASCII characters")
			}
		}
		if AuthMode(c.AuthType) == AuthBearer && !bearerValue.MatchString(c.Token) {
			return errors.New("Enter the Bearer token only, without the Bearer prefix")
		}
	}
	return nil
}
func sameAuth(modeA, headerA, modeB, headerB string) bool {
	if AuthMode(modeA) != AuthMode(modeB) {
		return false
	}
	return AuthMode(modeA) != AuthAPIKey || APIKeyHeader(headerA) == APIKeyHeader(headerB)
}

// Windows Credential Manager limits the full UTF-8 credential blob to 2560 bytes.
// Use the same limit on every OS so a supported profile behaves consistently.
const MaxCredentialRecord = 2560

func validateCredentialRecord(c Credentials, endpoint string) error {
	c.Endpoint = endpoint
	data, err := json.Marshal(c)
	if err != nil || len(data) > MaxCredentialRecord {
		return errors.New("Credentials and endpoint exceed the 2560-byte portable vault limit; use a shorter token or API key")
	}
	return nil
}

// ValidateCredentials binds vault contents to the selected authentication mode.
func ValidateCredentials(c Connection, cred Credentials) error {
	if err := validateAuthConfig(c.AuthType, c.AuthHeader); err != nil {
		return err
	}
	if !sameAuth(c.AuthType, c.AuthHeader, cred.AuthType, cred.AuthHeader) {
		return errors.New("Authentication method or API key header changed; enter credentials again")
	}
	if cred.Endpoint != "" && cred.Endpoint != c.URL {
		return errors.New("Saved credentials belong to a different endpoint; enter them again")
	}
	if err := validateCredentialValue(cred); err != nil {
		return err
	}
	return validateCredentialRecord(cred, c.URL)
}
func (c Credentials) applyAuth(header http.Header) error {
	if err := validateCredentialValue(c); err != nil {
		return err
	}
	// Do not carry another authentication scheme from a caller or SDK request.
	for _, name := range []string{"Authorization", "Proxy-Authorization", "X-API-Key", "Api-Key", "X-Auth-Token"} {
		header.Del(name)
	}
	switch AuthMode(c.AuthType) {
	case AuthBasic:
		r := &http.Request{Header: header}
		r.SetBasicAuth(c.Login, c.Password)
	case AuthBearer:
		header.Set("Authorization", "Bearer "+c.Token)
	case AuthAPIKey:
		header.Set(APIKeyHeader(c.AuthHeader), c.Token)
	}
	return nil
}
