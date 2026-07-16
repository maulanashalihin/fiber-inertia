// Package fiberinertia implements an Inertia.js server-side adapter for
// Go Fiber — native func(c *fiber.Ctx) error, no http.Handler bridge.
//
// Inertia.js lets you build single-page apps using classic server-side
// routing and controllers without building an API. This adapter handles
// the server side of the Inertia protocol:
//   - Auto-detects Inertia XHR requests vs initial full-page loads
//   - Returns JSON for Inertia requests, HTML for initial loads
//   - Asset versioning (409 Conflict on version mismatch)
//   - Shared / global props
//   - Partial reloads (X-Inertia-Partial-Data / Except)
//   - Internal (303) and external (409+X-Inertia-Location) redirects
//
// Minimal example:
//
//	package main
//
//	import (
//	    "github.com/gofiber/fiber/v2"
//	    fiberinertia "github.com/maulanashalihin/fiber-inertia"
//	)
//
//	func main() {
//	    app := fiber.New()
//
//	    inertia := fiberinertia.New(fiberinertia.Config{
//	        Version: "1.0",
//	    })
//
//	    app.Use(inertia.Middleware())
//
//	    app.Get("/dashboard", func(c *fiber.Ctx) error {
//	        return inertia.Render(c, "Dashboard", fiber.Map{
//	            "user": fiber.Map{"name": "Maulana"},
//	        })
//	    })
//
//	    app.Listen(":3000")
//	}
package fiberinertia

import (
	"encoding/json"
	"fmt"
	"html"

	"github.com/gofiber/fiber/v2"
)

// Inertia is the main adapter struct. Create one via New().
type Inertia struct {
	version     string
	renderFunc  func(c *fiber.Ctx, page *Page) error
	sharedProps []sharedProp
}

// Config holds the Inertia adapter configuration.
type Config struct {
	// Version is the current asset version. When the client sends a
	// different X-Inertia-Version header, a 409 Conflict response is
	// returned with X-Inertia-Location set, triggering a full page reload.
	//
	// Common values: a file hash, git commit hash, or build timestamp.
	// Leave empty to disable version checking.
	Version string

	// Render is an optional function that renders the root HTML page for
	// initial (non-Inertia) page loads.
	//
	// If nil, the default root template is used (see DefaultRootTemplate).
	// The default template renders:
	//   <div id="app" data-page='{pageJSON}'></div>
	//   <script type="module" src="/assets/main.js"></script>
	//
	// Override this to:
	//   - Include CSS/JS assets from Vite, Webpack, etc.
	//   - Set a dynamic page title
	//   - Inject CSRF tokens or other meta tags
	Render func(c *fiber.Ctx, page *Page) error
}

// Page represents the Inertia page object sent to the client on every
// response. See https://inertiajs.com/the-protocol#the-page-object
type Page struct {
	// Component is the JavaScript page component name (e.g. "Dashboard").
	Component string `json:"component"`

	// Props is the page data passed to the component.
	Props fiber.Map `json:"props"`

	// URL is the current page URL.
	URL string `json:"url"`

	// Version is the current asset version identifier.
	Version string `json:"version,omitempty"`

	// EncryptHistory indicates the page's history entry should be
	// encrypted (server-driven history encryption).
	EncryptHistory *bool `json:"encryptHistory,omitempty"`

	// ClearHistory tells the client to clear history state.
	ClearHistory *bool `json:"clearHistory,omitempty"`

	// MergeProps lists props that should be merged (appended) rather than
	// replaced on the client during navigation (e.g. for infinite scroll).
	MergeProps []string `json:"mergeProps,omitempty"`

	// PrependProps lists props that should be prepended during navigation.
	PrependProps []string `json:"prependProps,omitempty"`

	// DeepMergeProps lists props that should be deep merged during navigation.
	DeepMergeProps []string `json:"deepMergeProps,omitempty"`

	// MatchPropsOn controls how merge/prepend props are deduplicated.
	MatchPropsOn []string `json:"matchPropsOn,omitempty"`

	// ScrollProps carries infinite-scroll scroll position data.
	ScrollProps fiber.Map `json:"scrollProps,omitempty"`

	// DeferredProps lists props that are resolved asynchronously on
	// the client side after the page renders.
	DeferredProps fiber.Map `json:"deferredProps,omitempty"`

	// RescuedProps lists deferred prop keys that failed to resolve and
	// were rescued server-side.
	RescuedProps []string `json:"rescuedProps,omitempty"`

	// SharedProps lists top-level prop keys registered via Share().
	// Used by the client to carry shared props during instant visits.
	SharedProps []string `json:"sharedProps,omitempty"`

	// OnceProps are props that resolve only once and are cached on the
	// client for subsequent pages. Each key maps to an object containing
	// the prop name and optional expiry.
	OnceProps fiber.Map `json:"onceProps,omitempty"`
}

// sharedProp is either a static value or a per-request function.
type sharedProp struct {
	key   string
	value interface{}
	fn    func(c *fiber.Ctx) interface{}
}

// New creates a new Inertia adapter with the given configuration.
func New(cfg Config) *Inertia {
	return &Inertia{
		version:    cfg.Version,
		renderFunc: cfg.Render,
	}
}

// ---------------------------------------------------------------------------
// Shared / global props
// ---------------------------------------------------------------------------

// Share registers a static global prop that will be included in every page
// render. If a key is already registered, it is overwritten. Page-specific
// props with the same key override shared props.
func (i *Inertia) Share(key string, value interface{}) {
	i.removeShared(key)
	i.sharedProps = append(i.sharedProps, sharedProp{key: key, value: value})
}

// ShareFunc registers a dynamic global prop that is resolved per-request.
// The fn receives the current fiber.Ctx and must return the prop value.
func (i *Inertia) ShareFunc(key string, fn func(c *fiber.Ctx) interface{}) {
	i.removeShared(key)
	i.sharedProps = append(i.sharedProps, sharedProp{key: key, fn: fn})
}

func (i *Inertia) removeShared(key string) {
	for idx, sp := range i.sharedProps {
		if sp.key == key {
			i.sharedProps = append(i.sharedProps[:idx], i.sharedProps[idx+1:]...)
			return
		}
	}
}

// ---------------------------------------------------------------------------
// Core: Render
// ---------------------------------------------------------------------------

// Render sends an Inertia-compatible response.
//
// For Inertia XHR requests (detected via the X-Inertia header), it returns
// a JSON page object. For initial full-page loads (browser navigation), it
// returns the root HTML document with the page object embedded.
//
// Shared props are always included in every response, even during partial
// reloads. Partial reload filtering applies only to page-specific props.
func (i *Inertia) Render(c *fiber.Ctx, component string, props fiber.Map) error {
	// Apply partial reload filtering to page-specific props first.
	// Shared props are merged afterwards so they survive filtering.
	if i.isPartialRequest(c, component) {
		props = i.filterPartialProps(c, props)
	}

	mergedProps := i.mergeSharedProps(c, props)

	page := &Page{
		Component: component,
		Props:     mergedProps,
		URL:       string(c.Request().URI().RequestURI()),
		Version:   i.version,
	}

	// Shared props keys for the client (used for instant visits)
	if len(i.sharedProps) > 0 {
		keys := make([]string, 0, len(i.sharedProps))
		for _, sp := range i.sharedProps {
			keys = append(keys, sp.key)
		}
		page.SharedProps = keys
	}

	if c.Get("X-Inertia") == "true" {
		return i.renderJSON(c, page)
	}
	return i.renderHTML(c, page)
}

// mergeSharedProps merges registered global props first, then page-specific
// props override them. Returns a new map — does not mutate the input.
func (i *Inertia) mergeSharedProps(c *fiber.Ctx, props fiber.Map) fiber.Map {
	if len(i.sharedProps) == 0 && props == nil {
		return make(fiber.Map)
	}

	result := make(fiber.Map, len(i.sharedProps)+len(props))

	for _, sp := range i.sharedProps {
		if sp.fn != nil {
			result[sp.key] = sp.fn(c)
		} else {
			result[sp.key] = sp.value
		}
	}

	for k, v := range props {
		result[k] = v
	}

	return result
}

// ---------------------------------------------------------------------------
// JSON response
// ---------------------------------------------------------------------------

// renderJSON returns the page object as JSON for Inertia XHR requests.
func (i *Inertia) renderJSON(c *fiber.Ctx, page *Page) error {
	c.Set("X-Inertia", "true")
	c.Set("X-Inertia-Version", i.version)
	c.Set(fiber.HeaderCacheControl, "no-store, no-cache, must-revalidate, private")
	c.Set(fiber.HeaderVary, "X-Inertia")
	c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)

	return c.JSON(page)
}

// ---------------------------------------------------------------------------
// HTML response
// ---------------------------------------------------------------------------

// DefaultRootTemplate is the default HTML document used for initial page
// loads when no custom Render function is provided in Config.
//
// Placeholders:
//   %s — page title (from props["_title"], or "Inertia" as fallback)
//   %s — JSON-encoded page object (HTML-escaped for data-page attribute)
const DefaultRootTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>%s - Inertia</title>
</head>
<body>
    <div id="app" data-page='%s'></div>
    <script type="module" src="/assets/main.js"></script>
</body>
</html>`

// renderHTML renders the root HTML page for initial full-page loads.
func (i *Inertia) renderHTML(c *fiber.Ctx, page *Page) error {
	if i.renderFunc != nil {
		return i.renderFunc(c, page)
	}

	// JSON-encode the page object and escape for HTML attribute
	jsonBytes, err := json.Marshal(page)
	if err != nil {
		return fmt.Errorf("fiberinertia: failed to marshal page: %w", err)
	}

	// Escape single quotes for safe embedding in data-page='...'
	jsonStr := html.EscapeString(string(jsonBytes))

	// Extract title from props, or use "Inertia"
	title := extractTitle(page.Props)

	body := fmt.Sprintf(DefaultRootTemplate, title, jsonStr)

	c.Set(fiber.HeaderCacheControl, "no-store, no-cache, must-revalidate, private")
	c.Set(fiber.HeaderVary, "X-Inertia")
	c.Set(fiber.HeaderContentType, "text/html; charset=utf-8")

	return c.SendString(body)
}

// extractTitle extracts the page title from props. It checks "_title" key
// first, then falls back to "title", then to "Inertia".
func extractTitle(props fiber.Map) string {
	for _, key := range []string{"_title", "title"} {
		if v, ok := props[key]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	return "Inertia"
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// BoolPtr is a helper that returns a pointer to a bool value.
// Useful for setting optional Page fields like EncryptHistory.
func BoolPtr(b bool) *bool {
	return &b
}
