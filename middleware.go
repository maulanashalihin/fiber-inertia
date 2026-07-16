package fiberinertia

import (
	"github.com/gofiber/fiber/v2"
)

// ---------------------------------------------------------------------------
// Middleware
//
// The Inertia middleware handles:
//
//  1. Asset version checking — compares X-Inertia-Version from the client
//     against the configured server version. On mismatch, returns 409 with
//     X-Inertia-Location to trigger a full page reload.
//
//  2. Vary header — sets Vary: X-Inertia so caches differentiate between
//     HTML and JSON responses for the same URL.
//
//  3. Shared props — injects global props (registered via Share/ShareFunc)
//     into the request context so they're available downstream. However,
//     shared props are actually merged in Render(), not the middleware.
//
// Place this middleware early in your Fiber app so version checking runs
// on every request (including non-Inertia requests for consistency).
// ---------------------------------------------------------------------------

// Middleware returns a Fiber handler that performs version checking and
// sets appropriate response headers for Inertia compatibility.
//
// Usage:
//
//	app := fiber.New()
//	inertia := fiberinertia.New(fiberinertia.Config{Version: "1.0"})
//	app.Use(inertia.Middleware())
func (i *Inertia) Middleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Only check version on Inertia XHR requests
		if c.Get("X-Inertia") == "true" {
			clientVersion := c.Get("X-Inertia-Version")

			// If server has a version configured and client doesn't match
			if i.version != "" && clientVersion != "" && clientVersion != i.version {
				url := string(c.Request().URI().RequestURI())
				c.Set("X-Inertia-Location", url)
				return c.SendStatus(fiber.StatusConflict)
			}
		}

		// Inertia middleware also handles the redirect fix for PUT/PATCH/DELETE:
		// After a form submission, the 303 response must have X-Inertia set
		// so the client's XHR follows the redirect.

		return c.Next()
	}
}

// ForceVersionCheck can be called from any handler to explicitly trigger
// a version mismatch check, returning 409 if the client version doesn't
// match. This is useful for API endpoints that bypass the main middleware.
func (i *Inertia) ForceVersionCheck(c *fiber.Ctx) (conflict bool, err error) {
	if i.version == "" {
		return false, nil
	}

	clientVersion := c.Get("X-Inertia-Version")
	if clientVersion == "" || clientVersion == i.version {
		return false, nil
	}

	url := string(c.Request().URI().RequestURI())
	c.Set("X-Inertia-Location", url)
	return true, c.SendStatus(fiber.StatusConflict)
}
