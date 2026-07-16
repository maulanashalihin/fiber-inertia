package fiberinertia

import (
	"strings"

	"github.com/gofiber/fiber/v2"
)

// ---------------------------------------------------------------------------
// Partial reloads
//
// Inertia supports partial reloads — requesting a subset of props for the
// same page component to avoid re-fetching expensive data on every visit.
//
// Headers (sent by the client):
//
//	X-Inertia-Partial-Component  — the component name being partially reloaded
//	X-Inertia-Partial-Data       — comma-separated props to INCLUDE (omit others)
//	X-Inertia-Partial-Except     — comma-separated props to EXCLUDE (include rest)
//
// When both Data and Except are present, Except takes precedence.
// Partial reloads only apply when the target component matches the
// current page component.
// ---------------------------------------------------------------------------

// isPartialRequest returns true when the request is a partial reload for
// the given component.
func (i *Inertia) isPartialRequest(c *fiber.Ctx, component string) bool {
	return c.Get("X-Inertia-Partial-Component") == component
}

// filterPartialProps returns only the props requested in a partial reload.
// When no partial headers are present (or component doesn't match), it
// returns props unchanged.
func (i *Inertia) filterPartialProps(c *fiber.Ctx, props fiber.Map) fiber.Map {
	partialExcept := c.Get("X-Inertia-Partial-Except")
	partialData := c.Get("X-Inertia-Partial-Data")

	// 1. X-Inertia-Partial-Except — remove these keys, keep everything else.
	if partialExcept != "" {
		exclude := splitTrimSet(partialExcept)
		result := make(fiber.Map, len(props))
		for k, v := range props {
			if !exclude[k] {
				result[k] = v
			}
		}
		return result
	}

	// 2. X-Inertia-Partial-Data — keep only these keys.
	if partialData != "" {
		include := splitTrimSet(partialData)
		result := make(fiber.Map, len(include))
		for k, v := range props {
			if include[k] {
				result[k] = v
			}
		}
		return result
	}

	return props
}

// splitTrimSet splits s by comma, trims whitespace from each part, and
// returns a set (map[string]bool) for fast lookups. Returns nil for empty s.
func splitTrimSet(s string) map[string]bool {
	parts := strings.Split(s, ",")
	set := make(map[string]bool, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			set[p] = true
		}
	}
	return set
}
