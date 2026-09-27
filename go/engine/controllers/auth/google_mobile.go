package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	config "tls-rest/go/app/constants"
	"tls-rest/go/engine/controllers/db/pgdb"
	"tls-rest/go/engine/controllers/functions"
	"tls-rest/go/engine/modules/users"
)

// Google sign-in for external apps (the Android app). The app signs in with Credential Manager / Google Sign-In using the WEB
// client id (GOOGLE_ID) as its server client id, gets an ID token, and posts it to /api/auth/oauth as {provider:"google", id_token}.
// The token is verified with Google's tokeninfo endpoint and must have been issued for this server (aud). Its authorized party
// (azp, the app's own client id) is matched against the External Apps module (external_config.client_id), whose row also
// decides the user group the sign-in joins.

const googleTokenInfo = "https://oauth2.googleapis.com/tokeninfo"

var googleHTTP = &http.Client{Timeout: 8 * time.Second}

type googleTokenClaims struct {
	Issuer          string `json:"iss"`
	Audience        string `json:"aud"`
	AuthorizedParty string `json:"azp"`
	Subject         string `json:"sub"`
	Email           string `json:"email"`
	EmailVerified   string `json:"email_verified"`
	GivenName       string `json:"given_name"`
	FamilyName      string `json:"family_name"`
	Picture         string `json:"picture"`
	Expires         string `json:"exp"`
}

// externalClientIDs are the Google OAuth client ids registered on external_config rows (the external apps allowed to sign users in).
func externalClientIDs(ctx context.Context) []string {
	db, err := pgdb.GetInstanceCtx(ctx)
	if err != nil {
		return nil
	}
	rows, err := db.GetAll(`SELECT client_id FROM external_config WHERE provider = 'google' AND COALESCE(client_id, '') <> ''`)
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, functions.Coerce[string](r["client_id"]))
	}
	return out
}

// tokenInfo asks Google to validate a token and describe it; Google checks the signature and expiry.
func tokenInfo(ctx context.Context, param, token string) (*googleTokenClaims, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, googleTokenInfo+"?"+param+"="+url.QueryEscape(token), nil)
	if err != nil {
		return nil, err
	}
	resp, err := googleHTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google tokeninfo: status %d", resp.StatusCode)
	}
	var c googleTokenClaims
	if err := json.NewDecoder(resp.Body).Decode(&c); err != nil {
		return nil, err
	}
	return &c, nil
}

// audienceOK reports whether the token was issued for this server's web client or a client registered in external_config.
func audienceOK(ctx context.Context, c *googleTokenClaims) bool {
	if c.Audience == config.GoogleID {
		return true
	}
	for _, id := range externalClientIDs(ctx) {
		if c.Audience == id {
			return true
		}
	}
	return false
}

// registeredClient returns the external_config client id the token was issued to (its azp or aud), or "" for the web client.
func registeredClient(ctx context.Context, c *googleTokenClaims) string {
	for _, id := range externalClientIDs(ctx) {
		if c.AuthorizedParty == id || c.Audience == id {
			return id
		}
	}
	return ""
}

// verifyGoogleAccessToken rejects Google access tokens issued to some other app (token substitution) and returns the
// registered external client the token was issued to ("" for the web client).
func verifyGoogleAccessToken(ctx context.Context, accessToken string) (string, error) {
	c, err := tokenInfo(ctx, "access_token", accessToken)
	if err != nil {
		return "", err
	}
	if !audienceOK(ctx, c) {
		return "", errors.New("google access token was not issued for this app")
	}
	return registeredClient(ctx, c), nil
}

// GoogleIDTokenLogin signs in (or up) a user from an external app's Google ID token and returns the local user id and username.
// A token issued to a client registered in external_config also puts the user in that row's group.
func GoogleIDTokenLogin(ctx context.Context, idToken string) (int, string, error) {
	c, err := tokenInfo(ctx, "id_token", idToken)
	if err != nil {
		return 0, "", err
	}
	if c.Issuer != "accounts.google.com" && c.Issuer != "https://accounts.google.com" {
		return 0, "", errors.New("google id token: bad issuer")
	}
	if !audienceOK(ctx, c) {
		return 0, "", errors.New("google id token: wrong audience")
	}
	if c.Subject == "" {
		return 0, "", errors.New("google id token: no subject")
	}

	acc := &users.OAuthAccount{
		Provider:      "google",
		ProviderID:    c.Subject,
		Email:         strings.ToLower(strings.TrimSpace(c.Email)),
		EmailVerified: c.EmailVerified == "true",
		FirstName:     c.GivenName,
		LastName:      c.FamilyName,
		Image:         c.Picture,
	}
	// Only a client registered in external_config proves the sign-in came from that app.
	acc.ClientID = registeredClient(ctx, c)

	uid, username, err := users.FindOrCreateOAuthUser(ctx, acc)
	if err != nil {
		return 0, "", err
	}
	return int(uid), username, nil
}
