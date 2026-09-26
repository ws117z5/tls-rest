// Package submenu maps menu submenu names to their icons; /api/modules returns the map so the client holds no icon table of its own.
package submenu

import (
	"maps"
	"sync"
)

var (
	mu    sync.RWMutex
	icons = map[string]string{
		"engine":   "engine",
		"logs":     "log",
		"users":    "users",
		"shop":     "terms",
		"games":    "games",
		"tools":    "tools",
		"External": "external",
		"Legal":    "legal",
	}
)

// Register sets (or overrides) the icon for a submenu name; call from init().
func Register(name, icon string) {
	mu.Lock()
	defer mu.Unlock()
	icons[name] = icon
}

// Icons returns a copy of the submenu name -> icon map.
func Icons() map[string]string {
	mu.RLock()
	defer mu.RUnlock()
	return maps.Clone(icons)
}
