package module

import (
	"sort"
	"sync"
)

// Menu metadata registries. Modules and pages push their menu-facing metadata
// here at Initialize() time; the /api/modules and /api/pages controllers read it
// (rights/session-filtered) so the client menu is driven by what's actually
// registered in Go — no separate go.config.json to keep in sync.

// ModuleMenuMeta is the menu-facing description of a registered module. The
// endpoint is always "/"+ID, so it isn't stored.
type ModuleMenuMeta struct {
	ID          string
	Name        string
	Description string
	Order       int
	Icon        string
	// Submenu, when non-empty, groups this module under a named submenu in the
	// menu response; empty means it sits at the top level ("head").
	Submenu string
	// ReadOnly modules expose only list/view in the menu (no create/edit/delete),
	// matching modules whose write handlers are overridden to 405.
	ReadOnly bool
	Hidden   bool
}

// PageMenuMeta is the menu-facing description of a registered page.
type PageMenuMeta struct {
	ID            string
	Name          string
	RequiresAuth  bool
	RequiresAdmin bool
	Order         int
	Icon          string
	// Submenu groups this page under a named submenu; empty = top level ("head").
	Submenu string
}

var (
	menuMu     sync.RWMutex
	moduleMenu = map[string]ModuleMenuMeta{}
	pageMenu   = map[string]PageMenuMeta{}
)

func registerModuleMenu(m ModuleMenuMeta) {
	menuMu.Lock()
	moduleMenu[m.ID] = m
	menuMu.Unlock()
}

func registerPageMenu(p PageMenuMeta) {
	menuMu.Lock()
	pageMenu[p.ID] = p
	menuMu.Unlock()
}

// RegisteredModuleMenu returns all registered modules' menu metadata, ordered by
// Order then Name.
func RegisteredModuleMenu() []ModuleMenuMeta {
	menuMu.RLock()
	out := make([]ModuleMenuMeta, 0, len(moduleMenu))
	for _, m := range moduleMenu {
		out = append(out, m)
	}
	menuMu.RUnlock()
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Order != out[j].Order {
			return out[i].Order < out[j].Order
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// RegisteredPageMenu returns all registered pages' menu metadata, ordered by
// Order then Name.
func RegisteredPageMenu() []PageMenuMeta {
	menuMu.RLock()
	out := make([]PageMenuMeta, 0, len(pageMenu))
	for _, p := range pageMenu {
		out = append(out, p)
	}
	menuMu.RUnlock()
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Order != out[j].Order {
			return out[i].Order < out[j].Order
		}
		return out[i].Name < out[j].Name
	})
	return out
}
