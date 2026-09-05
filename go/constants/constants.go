package constants

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
)

// ConfigType is the parsed go.config.json — the app's declared modules. The
// field schema stays in Go (module.NewField); this only declares that a module
// exists, what it is, and the router endpoint it is exposed on.
type ConfigType struct {
	Modules []ModuleParams `json:"modules"`
	Log     LogParams      `json:"log"`
}

// LogParams configures the structured event logger (go/lib/log), mirroring the
// two sinks: writeToFile persists JSONL under ./logs, writeToDb inserts into the
// logs table (see the logs module). Applied at startup in main.
type LogParams struct {
	WriteToFile bool `json:"writeToFile"`
	WriteToDb   bool `json:"writeToDb"`
}

// AdditionalRights self explanitory
type AdditionalRights struct {
	Edit   string
	Create string
	Delete string
}

// ModuleParams config params of a module
type ModuleParams struct {
	Name             string           `json:"name"`
	Description      string           `json:"description"`
	Endpoint         string           `json:"endpoint"`
	RightsMask       string           `json:"rightsMask,omitempty"`
	AdditionalRights AdditionalRights `json:"additionalRights,omitempty"`
}

var (
	//JsHeader *TODO move to bson config file most of variables, here define structures
	JsHeader = []string{} //"/js/dist/platform.js"}

	//JsHeaderAttr = [][]string{"/js/dist/platform.js"}

	//JsFooter js footer array todo
	JsFooter = append(GetFiles([]string{"main.js"}), "/js/static/gl-matrix-min.js")

	//Css styles array
	Css = []string{"/css/bootstrap.min.css", "/css/index.css", "/css/index-cv.css", "/css/menu.css", "/css/theme-dark.css"}

	//Img Images array
	Img = []string{
		"/img/background.jpg",
		"/img/pano.jpg",
	}

	SQLPath       = "sql"
	SQLBackupPath = "backup"

	//MDb mongodb://foo:bar@localhost:27017
	MDb = Db{
		Addr:     Env("MONGO_ADDR", "mongodb://localhost:27017"),
		Timeout:  10,
		Database: "tls-rest",
	}

	//PDb TODO move everything related to passwords into another file excluded from git
	PDb = Db{
		Addr:     PDbAddr, //in private.go
		Timeout:  10,
		Database: "tls-rest",
		Password: PdbPass,
		User:     PdbUser,
	}

	//RDb TODO move
	RDb = Db{
		Addr: Env("REDIS_ADDR", "localhost:6379"),
	}

	// ---- Secrets / environment-specific config ----
	// Loaded from a .env file (see .env.example) or OS environment variables via
	// the helpers in env.go. Nothing sensitive is hard-coded here.
	//
	// REQUIRED: the server refuses to start (see ValidateRequired in main) if any
	// of these are missing.
	JWTSignature = RequireEnvBytes("JWT_SIGNATURE") // openssl rand -base64 32
	GoogleID     = RequireEnv("GOOGLE_ID")
	GoogleSecret = RequireEnv("GOOGLE_SECRET")
	PDbAddr      = RequireEnv("PG_ADDR")
	PdbUser      = RequireEnv("PG_USER")
	PdbPass      = RequireEnv("PG_PASSWORD")

	// OPTIONAL: feature-specific, with sane non-secret fallbacks.
	VKID     = EnvInt("VK_ID", 0)
	VKSecKey = Env("VK_SECRET_KEY", "")

	// OAuth providers (optional). A provider whose ID/secret is empty is simply
	// offered-but-unconfigured: attempting it redirects to /login?error=
	// provider_unconfigured rather than failing startup.
	FacebookID     = Env("FACEBOOK_ID", "")
	FacebookSecret = Env("FACEBOOK_SECRET", "")
	GithubID       = Env("GITHUB_ID", "")
	GithubSecret   = Env("GITHUB_SECRET", "")

	// GoogleURLBlank is the OOB (out-of-band) sentinel — not a host, so it is
	// not host-dependent and stays a constant.
	GoogleURLBlank = Env("GOOGLE_URL_BLANK", "urn:ietf:wg:oauth:2.0:oob")

	// NOTE: the former VKLink / GoogleURLLocal / LocalURL constants are gone.
	// Absolute app URLs (OAuth redirects, canonical links) are now derived
	// per-request from the host the client used — see go/lib/httpx.BaseURL and
	// the APP_HOSTS allowlist. This lets one build serve localhost, LAN and the
	// public domain without any hardcoded host.

	//Config a config file
	Config = new(ConfigType)
)

// GetProjectVersion extracts the version from go.mod build info,
// falling back to a default version string if unavailable.
func GetProjectVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "1.0.0"
	}

	// 1. If built via go install/module release, Main.Version contains the tag (e.g., "v1.2.3")
	if info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}

	// 2. If built from source/Git, extract the VCS revision (commit hash)
	for _, setting := range info.Settings {
		if setting.Key == "vcs.revision" {
			if len(setting.Value) >= 8 {
				return setting.Value[:8] // Short commit hash (e.g., "a1b2c3d4")
			}
			return setting.Value
		}
	}

	return "1.0.0"
}

// Call maps a slice of filenames to relative web paths with the project version query parameter
func GetFiles(files []string) []string {
	version := GetProjectVersion()
	result := make([]string, 0, len(files))

	for _, file := range files {
		found := false
		ext := filepath.Ext(file)                 // e.g. ".js"
		baseName := strings.TrimSuffix(file, ext) // e.g. "main"

		// 1. First, check for exact match (e.g., ./js/static/legacy-helper.js)
		exactPath := filepath.Join("./js/dist", file)
		if _, err := os.Stat(exactPath); err == nil {
			webPath := "/" + strings.TrimPrefix(filepath.ToSlash("./js/dist"), "./")
			result = append(result, fmt.Sprintf("%s/%s?v=%s", webPath, file, version))
			found = true
			break
		}

		// 2. If no exact match, search for hashed pattern (e.g., ./js/dist/main.*.js)
		pattern := filepath.Join("./js/dist", baseName+".*"+ext)
		matches, err := filepath.Glob(pattern)

		if err == nil && len(matches) > 0 {
			// Get the actual hashed filename on disk (e.g., "main.2e17295ca734.js")
			actualFilename := filepath.Base(matches[0])
			webPath := "/" + strings.TrimPrefix(filepath.ToSlash("./js/dist"), "./")

			// Returns: /js/dist/main.2e17295ca734.js?v=a1b2c3d4
			result = append(result, fmt.Sprintf("%s/%s?v=%s", webPath, actualFilename, version))
			found = true
			break
		}

		// Fallback if not found on disk
		if !found {
			result = append(result, fmt.Sprintf("/js/dist/%s?v=%s", file, version))
		}
	}

	return result
}

// GetModule returns a module configuration by name.
func (obj *ConfigType) GetModule(name string) (ModuleParams, error) {
	for _, m := range obj.Modules {
		if m.Name == name {
			return m, nil
		}
	}
	return ModuleParams{}, errors.New("module was not found: " + name)
}

// Validate checks that every configured module has a registered Go definition
// (its field schema) and a usable endpoint. It returns all problems found.
func (obj *ConfigType) Validate(registered map[string]bool) []error {
	var errs []error
	for _, m := range obj.Modules {
		if m.Name == "" {
			errs = append(errs, errors.New("go.config.json: module entry with empty name"))
			continue
		}
		if m.Endpoint == "" {
			errs = append(errs, fmt.Errorf("go.config.json: module %q has no endpoint", m.Name))
		}
		if !registered[m.Name] {
			errs = append(errs, fmt.Errorf("go.config.json: module %q has no Go definition (module.NewModule)", m.Name))
		}
	}
	return errs
}

func makeStruct(str string) map[string]interface{} {
	byt := []byte(str)
	var dat map[string]interface{}

	if err := json.Unmarshal(byt, &dat); err != nil {
		panic(err)
	}
	return dat
}

func main() {

	//includeFiles = []string{"/js/react/react.development.js", "/js/react/react-dom.development.js", "/js/app.jsx"}
}

func init() {
	loadConfig()
}

// loadConfig reads go.config.json from the project root (found by walking up to
// go.mod, the same mechanism as the .env loader) so it works regardless of the
// process working directory.
func loadConfig() {
	path := "go.config.json"
	if root := projectRoot(); root != "" {
		path = filepath.Join(root, "go.config.json")
	}

	b, err := os.ReadFile(path)
	if err != nil {
		fmt.Printf("constants: could not read %s: %v\n", path, err)
		return
	}
	if err := json.Unmarshal(b, Config); err != nil {
		fmt.Printf("constants: could not parse %s: %v\n", path, err)
	}
}
