package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	config "tls-rest/go/constants"

	"tls-rest/go/controllers/users"

	"tls-rest/go/lib"

	"net/http"

	"github.com/gorilla/mux"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

var googleConf *oauth2.Config
var stateString string

func init() {
	googleConf = &oauth2.Config{
		RedirectURL:  "https://localhost/users/Auth/GoogleCallback",
		ClientID:     config.GoogleID,
		ClientSecret: config.GoogleSecret,
		Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email", "profile", "email"},
		Endpoint:     google.Endpoint,
	}

	stateString, _ = lib.GetRandomHash(16)
}

// Auth simple auth
func Auth(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	vals := r.URL.Query()
	//user_id

	//https://oauth.vk.com/access_token?client_id=1&client_secret=H2Pk8htyFD8024mZaPHm&redirect_uri=http://mysite.ru&code=7a6fa4dff77a228eeda56603b8f53806c883f011c40b72630bb50df056f6479e52a
	if typeName, ok := vars["authType"]; ok {
		switch typeName {
		case "Vk":

			code := vals["code"][0]
			appID := strconv.Itoa(config.VKID)
			appSecret := config.VKSecKey
			appLink := config.VKLink

			url := "https://oauth.vk.com/access_token?client_id=" + appID + "&client_secret=" + appSecret + "&redirect_uri=" + appLink + "?code=" + code

			if resp, err := http.Get(url); err == nil {
				if body, err := io.ReadAll(resp.Body); err == nil {
					fmt.Println(bytes.NewBuffer(body).String())
				}
			}

		case "GoogleLogin":
			handleGoogleLogin(w, r)
		case "GoogleCallback":
			handleGoogleCallback(w, r)
		}
	}
}

func handleGoogleLogin(w http.ResponseWriter, r *http.Request) {
	url := googleConf.AuthCodeURL(stateString)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func handleGoogleCallback(w http.ResponseWriter, r *http.Request) {
	content, err := getUserInfo(r.FormValue("state"), r.FormValue("code"))
	if err != nil {
		fmt.Println(err.Error())
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}

	var GoogleAccount = new(users.GoogleAccount)
	err = json.Unmarshal(content, &GoogleAccount)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	fmt.Printf("%+v\n", GoogleAccount)
	//if everything went well we check user and register if not existing
	users.RegisterGoogleUser(GoogleAccount)

	fmt.Fprintf(w, "Content: %s\n", content)
}

func getUserInfo(state string, code string) ([]byte, error) {
	if state != stateString {
		return nil, fmt.Errorf("invalid oauth state")
	}
	token, err := googleConf.Exchange(oauth2.NoContext, code)
	if err != nil {
		return nil, fmt.Errorf("code exchange failed: %s", err.Error())
	}
	response, err := http.Get("https://www.googleapis.com/oauth2/v2/userinfo?access_token=" + token.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("failed getting user info: %s", err.Error())
	}
	defer response.Body.Close()
	contents, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed reading response body: %s", err.Error())
	}
	return contents, nil
}
