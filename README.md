# fiber-inertia

**Inertia.js server-side adapter for Go Fiber** — native `func(c *fiber.Ctx) error`, no `http.Handler` bridge.

[![CI](https://github.com/maulanashalihin/fiber-inertia/actions/workflows/ci.yml/badge.svg)](https://github.com/maulanashalihin/fiber-inertia/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/maulanashalihin/fiber-inertia.svg)](https://pkg.go.dev/github.com/maulanashalihin/fiber-inertia)
[![Go Report Card](https://goreportcard.com/badge/github.com/maulanashalihin/fiber-inertia)](https://goreportcard.com/report/github.com/maulanashalihin/fiber-inertia)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

`fiber-inertia` implements the [Inertia.js protocol](https://inertiajs.com/the-protocol) natively on Go Fiber. It auto-detects Inertia XHR requests versus initial full-page loads, returning JSON for the former and a root HTML document for the latter — all with zero dependencies beyond Fiber itself.

Supports **Inertia v3 protocol**: asset versioning, partial reloads, shared props, lazy/deferred props, merge props, once props, encrypted history, and external/internal redirects.

---

## Features

- ✅ **Auto-detect** — `X-Inertia` header check: JSON for XHR, HTML for initial load
- ✅ **Page object** — `{ component, props, url, version }` as specified by the protocol
- ✅ **Asset versioning** — 409 Conflict + `X-Inertia-Location` on version mismatch
- ✅ **Shared props** — static (`Share`) and dynamic per-request (`ShareFunc`)
- ✅ **Partial reloads** — `X-Inertia-Partial-Data`, `X-Inertia-Partial-Except`, `X-Inertia-Partial-Component`
- ✅ **Internal redirects** — 303 See Other for form submissions
- ✅ **External redirects** — 409 Conflict + `X-Inertia-Location` for full page navigations
- ✅ **Back navigation** — Referer-based `Back()` with configurable fallback
- ✅ **Lazy/deferred props** — `Page.DeferredProps` configuration for async data
- ✅ **Merge/Prepend/Deep-merge props** — `Page.MergeProps`, `Page.PrependProps`, `Page.DeepMergeProps`
- ✅ **Once props** — `Page.OnceProps` for one-time resolved data
- ✅ **Custom root template** — override `Config.Render` for Vite, Webpack, or any asset pipeline
- ✅ **No adaptor** — pure `func(c *fiber.Ctx) error`, no `http.Handler` bridge
- ✅ **Minimal dependencies** — only `github.com/gofiber/fiber/v2`

---

## Installation

```bash
go get github.com/maulanashalihin/fiber-inertia
```

---

## Quick Start

The simplest possible setup — 10 lines of Go:

```go
package main

import (
    "github.com/gofiber/fiber/v2"
    fiberinertia "github.com/maulanashalihin/fiber-inertia"
)

func main() {
    app := fiber.New()

    // 1. Create Inertia adapter
    inertia := fiberinertia.New(fiberinertia.Config{
        Version: "1.0",
    })

    // 2. Register middleware (version checking, Vary header)
    app.Use(inertia.Middleware())

    // 3. Render pages with Inertia
    app.Get("/", func(c *fiber.Ctx) error {
        return inertia.Render(c, "Home", fiber.Map{
            "title": "Welcome",
        })
    })

    app.Listen(":3000")
}
```

### What happens:

| Request type | `X-Inertia` header | Response |
|---|---|---|
| **Initial page load** (browser) | absent | Full HTML document with page data in `<div id="app" data-page='...'>` |
| **Inertia navigation** (SPA) | `true` | JSON `{ "component": "Home", "props": {...}, "url": "/" }` |

---

## Configuration

```go
inertia := fiberinertia.New(fiberinertia.Config{
    // Version is the current asset version. When the client sends a
    // different X-Inertia-Version header, a 409 Conflict response is
    // returned, triggering a full page reload.
    Version: "1.0",

    // Render is an optional function for custom root HTML rendering.
    // When nil, the DefaultRootTemplate is used (see below).
    Render: func(c *fiber.Ctx, page *fiberinertia.Page) error {
        // Your custom HTML shell — CSS, JS, meta tags, etc.
        return c.Type("html").SendString(fmt.Sprintf(`
            <!DOCTYPE html>
            <html>
            <head>
                <title>%s - My App</title>
                <link rel="stylesheet" href="/assets/app.css">
            </head>
            <body>
                <div id="app" data-page='%s'></div>
                <script type="module" src="/assets/main.js"></script>
            </body>
            </html>
        `, page.Props["title"], pageJSON))
    },
})
```

### Default root template

When no `Render` function is provided, the library uses `DefaultRootTemplate`:

```html
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{title} - Inertia</title>
</head>
<body>
    <div id="app" data-page='{pageJSON}'></div>
    <script type="module" src="/assets/main.js"></script>
</body>
</html>
```

The page title is extracted from `props["_title"]` or `props["title"]`. Override `Config.Render` to customise the HTML shell (e.g. for Vite HMR, different asset paths, or meta tags).

---

## Usage

### Rendering pages

```go
app.Get("/dashboard", func(c *fiber.Ctx) error {
    return inertia.Render(c, "Dashboard", fiber.Map{
        "user": fiber.Map{
            "name":  "Maulana",
            "email": "maulana@example.com",
        },
        "posts": []fiber.Map{
            {"id": 1, "title": "Hello World"},
        },
    })
})
```

### Shared props (global data)

Props that should be available on every page:

```go
// Static shared prop
inertia.Share("appName", "Laju")

// Dynamic shared prop (resolved per-request)
inertia.ShareFunc("user", func(c *fiber.Ctx) interface{} {
    // Fetch user from session, database, etc.
    return getCurrentUser(c)
})

// Page-specific props override shared props with the same key
app.Get("/login", func(c *fiber.Ctx) error {
    return inertia.Render(c, "Login", fiber.Map{
        "user": nil,  // Overrides the shared "user" prop
    })
})
```

### Redirects

```go
// Internal redirect (303 See Other) — Inertia follows via XHR
app.Post("/login", func(c *fiber.Ctx) error {
    // ... authenticate user ...
    return inertia.Redirect(c, "/dashboard")
})

// External redirect (409 + X-Inertia-Location) — full page navigation
app.Get("/logout", func(c *fiber.Ctx) error {
    // ... clear session ...
    return inertia.Location(c, "https://google.com")
})

// Back — go to previous page (from Referer header)
app.Post("/cancel", func(c *fiber.Ctx) error {
    return inertia.Back(c)          // no Referer → "/"
    return inertia.Back(c, "/home") // no Referer → "/home"
})
```

### Partial reloads

No extra code needed — the library handles partial reload requests automatically:

```go
app.Get("/dashboard", func(c *fiber.Ctx) error {
    return inertia.Render(c, "Dashboard", fiber.Map{
        "users":        fetchAllUsers(),        // Expensive — only sent when requested
        "notifications": fetchNotifications(),  // Included by default
    })
})
```

When the client sends `X-Inertia-Partial-Component: Dashboard` and `X-Inertia-Partial-Data: users`, only `users` is returned. The `notifications` prop is filtered out.

### Lazy / Deferred props

Props that should be loaded asynchronously on the client:

```go
app.Get("/dashboard", func(c *fiber.Ctx) error {
    return inertia.Render(c, "Dashboard", fiber.Map{
        "user":   currentUser,
        "stats":  nil,  // Placeholder — loaded asynchronously
    })
})
```

Set `Page.DeferredProps` metadata to tell the client which keys are deferred.

### Merge props (infinite scroll)

```go
app.Get("/posts", func(c *fiber.Ctx) error {
    page := &fiberinertia.Page{
        Component:  "Posts",
        Props:      fiber.Map{"posts": fetchPosts(page)},
        URL:        c.OriginalURL(),
        MergeProps: []string{"posts"},
    }
    // Use raw JSON rendering for full control
})
```

---

## Version helpers

```go
import fiberinertia "github.com/maulanashalihin/fiber-inertia"

// Auto version from file content
version, _ := fiberinertia.VersionFromFile("version.txt")

// Auto version from file hash (cache busting)
version, _ := fiberinertia.VersionFromFileHash("dist/assets/main.js")

// Version from environment variable
version := fiberinertia.VersionFromEnv("APP_VERSION")
```

---

## Example with Vite + Svelte

```go
package main

import (
    "fmt"
    "os"
    "strings"

    "github.com/gofiber/fiber/v2"
    fiberinertia "github.com/maulanashalihin/fiber-inertia"
)

func main() {
    app := fiber.New()

    // Read Vite port for dev server (or get from env)
    vitePort := getVitePort()
    isDev := vitePort != ""

    // Read production manifest
    manifest := loadManifest("dist/.vite/manifest.json")

    inertia := fiberinertia.New(fiberinertia.Config{
        Version: "1.0",
        Render: func(c *fiber.Ctx, page *fiberinertia.Page) error {
            pageJSON, _ := json.Marshal(page)

            var html string
            if isDev {
                html = fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><meta charset="UTF-8"><title>%s</title></head>
<body>
<div id="app" data-page='%s'></div>
<script type="module" src="%s/@vite/client"></script>
<script type="module" src="%s/src/main.ts"></script>
</body>
</html>`, page.Props["title"], pageJSON, vitePort, vitePort)
            } else {
                html = fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><meta charset="UTF-8"><title>%s</title>
<link rel="stylesheet" href="%s"></head>
<body>
<div id="app" data-page='%s'></div>
<script type="module" src="%s"></script>
</body>
</html>`, page.Props["title"], manifest["src/main.ts"].CSS[0], pageJSON, manifest["src/main.ts"].File)
            }

            return c.Type("html").SendString(html)
        },
    })

    app.Use(inertia.Middleware())
    app.Get("/", func(c *fiber.Ctx) error {
        return inertia.Render(c, "Home", fiber.Map{
            "title": "Home",
        })
    })

    app.Listen(":3000")
}
```

---

## Inertia Protocol Compliance

| Feature | Supported | Notes |
|---------|-----------|-------|
| **`X-Inertia` request header** | ✅ | Auto-detected |
| **`X-Inertia-Version` request header** | ✅ | Compared against `Config.Version` |
| **`X-Inertia-Partial-Component`** | ✅ | Enables partial reload |
| **`X-Inertia-Partial-Data`** | ✅ | Filter props to only these keys |
| **`X-Inertia-Partial-Except`** | ✅ | Filter props to exclude these keys |
| **`X-Inertia-Reset`** | ✅ | Pass through in page object |
| **`X-Inertia-Error-Bag`** | ✅ | Pass through in page object |
| **`X-Inertia-Except-Once-Props`** | ✅ | Once-prop expiry handling |
| **`X-Inertia` response header** | ✅ | Set on JSON responses |
| **`X-Inertia-Version` response header** | ✅ | Set on JSON responses |
| **`X-Inertia-Location` response header** | ✅ | 409 + Location for external redirects |
| **`X-Inertia-Redirect` response header** | ✅ | 409 for fragment redirects |
| **`Vary: X-Inertia` response header** | ✅ | Helps caching |
| **200 OK** | ✅ | Successful responses |
| **303 See Other** | ✅ | After POST/PUT/PATCH/DELETE |
| **409 Conflict** | ✅ | Version mismatch + external redirects |
| **422 Unprocessable Entity** | ✅ | Validation errors (via `c.Status()`) |
| **HTML response (initial load)** | ✅ | `<div id="app" data-page='{...}'>` |
| **JSON response (XHR)** | ✅ | `{component, props, url, version}` |
| **Page object** | ✅ | Full page object with all optional fields |
| **Shared props** | ✅ | `Share()` / `ShareFunc()` |
| **Partial reloads** | ✅ | Automatic in `Render()` |
| **Lazy/Deferred props** | ✅ | `Page.DeferredProps` |
| **Merge props** | ✅ | `Page.MergeProps` |
| **Once props** | ✅ | `Page.OnceProps` |
| **Encrypted history** | ✅ | `Page.EncryptHistory` |
| **Clear history** | ✅ | `Page.ClearHistory` |

---

## API Reference

### `New(cfg Config) *Inertia`

Creates a new Inertia adapter. See [Configuration](#configuration) for `Config` details.

### `(*Inertia) Render(c *fiber.Ctx, component string, props fiber.Map) error`

Sends an Inertia response. Auto-detects JSON vs HTML based on `X-Inertia` header.

### `(*Inertia) Share(key string, value interface{})`

Registers a static global prop available on every page.

### `(*Inertia) ShareFunc(key string, fn func(*fiber.Ctx) interface{})`

Registers a dynamic global prop resolved per-request.

### `(*Inertia) Redirect(c *fiber.Ctx, url string) error`

Sends a 303 See Other redirect to an internal route.

### `(*Inertia) Location(c *fiber.Ctx, url string) error`

Sends a 409 Conflict with `X-Inertia-Location` for external redirects.

### `(*Inertia) Back(c *fiber.Ctx, defaultURL ...string) error`

Redirects to the previous page using the Referer header.

### `(*Inertia) Middleware() fiber.Handler`

Returns a middleware that checks asset versions and sets proper headers.

### `(*Inertia) ForceVersionCheck(c *fiber.Ctx) (conflict bool, err error)`

Explicitly check version mismatch from any handler.

### Version helpers

- `VersionFromFile(path string) (string, error)` — read version from file
- `VersionFromFileHash(path string) (string, error)` — SHA-256 hash of file
- `VersionFromEnv(key string) string` — read version from env var

---

## Testing

```bash
go test ./...
```

Tests cover:
- JSON vs HTML response auto-detection
- Asset version matching and mismatch (409)
- Partial reload filtering (Data, Except, both)
- Shared props (static and dynamic)
- Internal and external redirects
- Back navigation with and without Referer
- Edge cases: nil props, empty version, missing headers

---

## License

MIT — see [LICENSE](LICENSE)
