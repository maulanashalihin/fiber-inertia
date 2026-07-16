package fiberinertia

import (
	"github.com/gofiber/fiber/v2"
)

// ---------------------------------------------------------------------------
// Redirects
//
// Inertia handles three types of redirects:
//
//  1. Internal redirect (Redirect) — standard server redirect to another
//     route within the app. Uses 303 See Other for POST/PUT/PATCH/DELETE
//     to prevent duplicate form submissions. The Inertia client intercepts
//     this and makes a subsequent XHR GET to the target URL.
//
//  2. External redirect (Location) — used when redirecting to an external
//     URL or a page that requires a full browser navigation. Returns
//     409 Conflict with X-Inertia-Location set to the target URL. The
//     Inertia client performs a window.location visit.
//
//  3. Back — navigates to the previous page using the Referer header.
// ---------------------------------------------------------------------------

// Redirect sends a 303 See Other redirect to an internal app route.
//
// For Inertia XHR requests, the client intercepts the 303 and follows it
// via XHR GET, maintaining SPA behaviour.
func (i *Inertia) Redirect(c *fiber.Ctx, url string) error {
	return c.Redirect(url, fiber.StatusSeeOther)
}

// Location sends a 409 Conflict response with X-Inertia-Location set,
// instructing the Inertia client to perform a full window.location visit.
//
// Use this for:
//   - Redirecting to external websites
//   - Redirecting after an asset version mismatch
//   - Any URL that should cause a full page reload
func (i *Inertia) Location(c *fiber.Ctx, url string) error {
	c.Set("X-Inertia-Location", url)
	return c.SendStatus(fiber.StatusConflict)
}

// Back redirects to the previous page using the Referer header.
// If no Referer is present, falls back to the given defaultURL
// (or "/" if empty).
func (i *Inertia) Back(c *fiber.Ctx, defaultURL ...string) error {
	fallback := "/"
	if len(defaultURL) > 0 && defaultURL[0] != "" {
		fallback = defaultURL[0]
	}

	referer := c.Get(fiber.HeaderReferer)
	if referer == "" {
		referer = fallback
	}

	return c.Redirect(referer, fiber.StatusSeeOther)
}

// GetReferer returns the value of the Referer header from the request.
// This is useful for custom redirect logic.
func GetReferer(c *fiber.Ctx) string {
	return c.Get(fiber.HeaderReferer)
}
