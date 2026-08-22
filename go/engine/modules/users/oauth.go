package users

import (
	"strings"

	"tls-rest/go/engine/controllers/db/pgdb"
)

// OAuthAccount is the provider-agnostic result of an OAuth sign-in, normalized
// from each provider's own user-info shape (Google, GitHub, Facebook, VK). The
// auth package fills it in and hands it to FindOrCreateOAuthUser.
type OAuthAccount struct {
	Provider   string // "google" | "github" | "facebook" | "vk"
	ProviderID string // the provider's stable user id
	Email      string // may be empty (some providers/users don't expose it)
	FirstName  string
	LastName   string
	Image      string
	Username   string // provider handle, if any (e.g. GitHub login) — used as a username fallback
}

// FindOrCreateOAuthUser resolves an OAuth sign-in to a local user id + display
// name, creating or linking as needed. Idempotent across repeated logins.
//
// Resolution:
//  1. match on (auth_provider, auth_provider_id) — the provider's own id;
//  2. else match on email and link this provider onto that existing account;
//  3. else create a new user.
func FindOrCreateOAuthUser(a *OAuthAccount) (int64, string, error) {
	db, err := pgdb.GetInstance()
	if err != nil {
		return 0, "", err
	}

	// 1) Already linked to this exact provider identity.
	if a.Provider != "" && a.ProviderID != "" {
		if row, e := db.GetOne(
			`SELECT id, user_name FROM users WHERE auth_provider = $1 AND auth_provider_id = $2 LIMIT 1`,
			a.Provider, a.ProviderID,
		); e == nil && row != nil {
			return pgdb.Coerce[int64](row["id"]), pgdb.Coerce[string](row["user_name"]), nil
		}
	}

	// 2) Known email → link this provider onto the existing account.
	if a.Email != "" {
		if row, e := db.GetOne(
			`SELECT id, user_name FROM users WHERE lower(email) = lower($1) LIMIT 1`, a.Email,
		); e == nil && row != nil {
			id := pgdb.Coerce[int64](row["id"])
			if a.Provider != "" && a.ProviderID != "" {
				_, _ = db.UpdateRow("users", map[string]interface{}{
					"auth_provider":    a.Provider,
					"auth_provider_id": a.ProviderID,
				}, "id", id)
			}
			return id, pgdb.Coerce[string](row["user_name"]), nil
		}
	}

	// 3) New user.
	username := oauthUsername(a)
	firstName := a.FirstName
	if firstName == "" {
		firstName = username // first_name is NOT NULL in the schema
	}

	id, err := db.InsertRow("users", map[string]interface{}{
		"user_name":        username,
		"first_name":       firstName,
		"last_name":        a.LastName,
		"email":            emptyToNil(a.Email),
		"image":            a.Image,
		"auth_provider":    emptyToNil(a.Provider),
		"auth_provider_id": emptyToNil(a.ProviderID),
	})
	if err != nil {
		return 0, "", err
	}
	return id, username, nil
}

// oauthUsername derives a non-empty display name: "First L." when a name is
// present, else the provider handle, else the email local-part, else a
// provider-scoped fallback.
func oauthUsername(a *OAuthAccount) string {
	lastInitial := ""
	if len(a.LastName) > 0 {
		lastInitial = " " + a.LastName[0:1] + "."
	}
	if name := strings.TrimSpace(a.FirstName + lastInitial); name != "" {
		return name
	}
	if a.Username != "" {
		return a.Username
	}
	if a.Email != "" {
		if i := strings.IndexByte(a.Email, '@'); i > 0 {
			return a.Email[:i]
		}
	}
	return a.Provider + "_" + a.ProviderID
}

// emptyToNil returns nil for an empty string so the column is stored NULL rather
// than an empty string (keeps the partial unique index and email lookups clean).
func emptyToNil(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
