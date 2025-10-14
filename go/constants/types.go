package constants

//import jwt "github.com/dgrijalva/jwt-go"

// ConfigData not sure yet
type ConfigData struct {
}

// TokenData jwt token data instance
type TokenData struct {
	UserAgent  string `json:"user_agent"`
	UserAdress string `json:"user_adress"`
	UserToken  string `json:"user_token"`
	ServiceID  int32  `json:"service_id"`
	UserID     string `json:"user_id"`
	//Claims     jwt.StandardClaims `json:claims`
}

// Db  db instance
type Db struct {
	User        string
	Password    string
	Port        string
	Addr        string
	Database    string
	DatabaseInt int
	Timeout     int
}

// Tmpl template data instance
type Tmpl struct {
	JsHeader  []string
	JsFooter  []string
	CssHeader []string
	Img       []string
	Title     string
	Body      map[string]interface{}
}
