package route

import (
	"net/http"
	"path/filepath"

	"tls-rest/go/engine/controllers/log"

	"github.com/gorilla/mux"
)

// StaticConfig represents a static directory configuration
type StaticConfig struct {
	URLPath   string // URL path prefix (e.g., "/css/")
	LocalPath string // Local directory path (e.g., "./css/")
	Name      string // Human-readable name for logging
}

// DefaultStaticConfigs defines all static directories
var DefaultStaticConfigs = []StaticConfig{
	{
		URLPath:   "/css/",
		LocalPath: "./css/",
		Name:      "CSS Files",
	},
	{
		URLPath:   "/js/",
		LocalPath: "./js/",
		Name:      "JavaScript Files",
	},
	{
		URLPath:   "/img/",
		LocalPath: "./img/",
		Name:      "Image Files",
	},
}

// RegisterStaticRoutes registers all static file routes with the given router
func RegisterStaticRoutes(router *mux.Router) {
	log.LogSystemEvent("Starting static route registration", log.LogLevelInfo,
		map[string]interface{}{
			"route_count": len(DefaultStaticConfigs),
		})

	for _, config := range DefaultStaticConfigs {
		registerSingleStaticRoute(router, config)
	}

	log.LogSystemEvent("Static route registration completed", log.LogLevelInfo,
		map[string]interface{}{
			"registered_routes": len(DefaultStaticConfigs),
		})
}

// registerSingleStaticRoute registers a single static directory
func registerSingleStaticRoute(router *mux.Router, config StaticConfig) {
	// Create file server for the directory
	fs := http.FileServer(http.Dir(config.LocalPath))

	// Strip the URL prefix and serve files
	handler := http.StripPrefix(config.URLPath, fs)

	// Register the route
	router.PathPrefix(config.URLPath).Handler(handler)

	log.LogSystemEvent("Static route registered", log.LogLevelInfo,
		map[string]interface{}{
			"name":       config.Name,
			"url_path":   config.URLPath,
			"local_path": config.LocalPath,
		})
}

// RegisterCustomStaticRoute allows registering additional static directories
func RegisterCustomStaticRoute(router *mux.Router, urlPath, localPath, name string) {
	config := StaticConfig{
		URLPath:   urlPath,
		LocalPath: localPath,
		Name:      name,
	}

	registerSingleStaticRoute(router, config)
}

// ValidateStaticPaths checks if all configured static directories exist
func ValidateStaticPaths() []string {
	var missing []string

	for _, config := range DefaultStaticConfigs {
		if !pathExists(config.LocalPath) {
			missing = append(missing, config.LocalPath)
		}
	}

	return missing
}

// pathExists checks if a path exists
func pathExists(path string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	if _, err := http.Dir(absPath).Open("/"); err != nil {
		return false
	}

	return true
}
