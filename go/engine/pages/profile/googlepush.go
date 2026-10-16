package profile

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"tls-rest/go/engine/controllers/db/pgdb"
	"tls-rest/go/engine/controllers/functions"

	"golang.org/x/oauth2"
)

// GoogleEventPush is one app event's data to push to its creator's Google
// Calendar — see events.go's pushToGoogleIfNeeded. GoogleEventID is empty
// for a first push (creates a new Google event); set, it updates that same
// Google event in place instead of creating a duplicate.
type GoogleEventPush struct {
	GoogleEventID   string
	Title           string
	Description     string
	StartsAt        time.Time
	DurationMinutes int
	Repeat          string // "" | "none" | "daily" | "weekly" | "monthly" | "annual"
}

// rruleFor translates this app's repeat field into a Google Calendar RRULE —
// nil for "none" (a single, non-recurring event).
func rruleFor(repeat string) []string {
	switch repeat {
	case "daily":
		return []string{"RRULE:FREQ=DAILY"}
	case "weekly":
		return []string{"RRULE:FREQ=WEEKLY"}
	case "monthly":
		return []string{"RRULE:FREQ=MONTHLY"}
	case "annual":
		return []string{"RRULE:FREQ=YEARLY"}
	default:
		return nil
	}
}

type gcalDateTime struct {
	DateTime string `json:"dateTime"`
}

type gcalEventBody struct {
	Summary     string       `json:"summary"`
	Description string       `json:"description,omitempty"`
	Start       gcalDateTime `json:"start"`
	End         gcalDateTime `json:"end"`
	Recurrence  []string     `json:"recurrence,omitempty"`
}

// googleClientFor builds an authenticated *http.Client for userID's
// connected account, refreshing and persisting the token if it's expired.
// Returns (nil, nil) — not an error — when the user has no connected
// account, so callers can treat "not connected" as a silent no-op.
func googleClientFor(ctx context.Context, db *pgdb.Db, userID int) (*http.Client, error) {
	row, err := db.GetOne(`SELECT access_token, refresh_token, token_expiry FROM google_calendar_accounts WHERE user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, nil
	}
	tok := &oauth2.Token{
		AccessToken:  functions.Coerce[string](row["access_token"]),
		RefreshToken: functions.Coerce[string](row["refresh_token"]),
		Expiry:       functions.Coerce[time.Time](row["token_expiry"]),
	}

	cfg := googleConfigForRefresh()
	fresh, err := cfg.TokenSource(ctx, tok).Token()
	if err != nil {
		return nil, err
	}
	if fresh.AccessToken != tok.AccessToken {
		_, _ = db.UpdateRow("google_calendar_accounts", map[string]interface{}{
			"access_token": fresh.AccessToken,
			"token_expiry": fresh.Expiry,
		}, "user_id", userID)
	}
	return cfg.Client(ctx, fresh), nil
}

// PushEventToGoogle creates or updates ev's copy on userID's connected
// Google Calendar. Returns ("", nil) — not an error — when the user has no
// connected account, so a caller that ignores a zero id simply skips it.
func PushEventToGoogle(ctx context.Context, userID int, ev GoogleEventPush) (string, error) {
	db, err := pgdb.GetInstanceCtx(ctx)
	if err != nil {
		return "", err
	}
	client, err := googleClientFor(ctx, db, userID)
	if err != nil || client == nil {
		return "", err
	}

	end := ev.StartsAt.Add(time.Duration(ev.DurationMinutes) * time.Minute)
	body := gcalEventBody{
		Summary:     ev.Title,
		Description: ev.Description,
		Start:       gcalDateTime{DateTime: ev.StartsAt.Format(time.RFC3339)},
		End:         gcalDateTime{DateTime: end.Format(time.RFC3339)},
		Recurrence:  rruleFor(ev.Repeat),
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	url := "https://www.googleapis.com/calendar/v3/calendars/primary/events"
	method := http.MethodPost
	if ev.GoogleEventID != "" {
		url += "/" + ev.GoogleEventID
		method = http.MethodPut
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("calendar API: unexpected status %d", resp.StatusCode)
	}

	var out struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if out.ID == "" {
		out.ID = ev.GoogleEventID
	}
	return out.ID, nil
}

// DeleteGoogleEvent removes an event from userID's connected Google
// Calendar. A 404/410 (already gone) is treated as success.
func DeleteGoogleEvent(ctx context.Context, userID int, googleEventID string) error {
	if googleEventID == "" {
		return nil
	}
	db, err := pgdb.GetInstanceCtx(ctx)
	if err != nil {
		return err
	}
	client, err := googleClientFor(ctx, db, userID)
	if err != nil || client == nil {
		return err
	}

	url := "https://www.googleapis.com/calendar/v3/calendars/primary/events/" + googleEventID
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 && resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusGone {
		return fmt.Errorf("calendar API: unexpected status %d", resp.StatusCode)
	}
	return nil
}
