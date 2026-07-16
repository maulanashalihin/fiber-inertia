package fiberinertia

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// setupTest creates a fresh Fiber app + Inertia adapter for testing.
func setupTest(opts ...func(*Config)) (*fiber.App, *Inertia) {
	cfg := Config{
		Version: "1.0",
	}
	for _, fn := range opts {
		fn(&cfg)
	}
	inertia := New(cfg)
	app := fiber.New()
	app.Use(inertia.Middleware())
	return app, inertia
}

// readBody reads the full response body.
func readBody(t *testing.T, resp *http.Response) []byte {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}
	resp.Body.Close()
	return body
}

// jsonBody decodes the response body as a Page.
func jsonBody(t *testing.T, body []byte) *Page {
	t.Helper()
	var page Page
	if err := json.Unmarshal(body, &page); err != nil {
		t.Fatalf("failed to decode JSON body: %v\nBody: %s", err, string(body))
	}
	return &page
}

// ---------------------------------------------------------------------------
// Render — JSON (Inertia XHR)
// ---------------------------------------------------------------------------

func TestRenderJSON_ReturnsPageObject(t *testing.T) {
	app, inertia := setupTest()

	app.Get("/dashboard", func(c *fiber.Ctx) error {
		return inertia.Render(c, "Dashboard", fiber.Map{
			"username": "maulana",
		})
	})

	req := httptest.NewRequest("GET", "/dashboard", nil)
	req.Header.Set("X-Inertia", "true")
	req.Header.Set("X-Inertia-Version", "1.0")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	page := jsonBody(t, readBody(t, resp))
	if page.Component != "Dashboard" {
		t.Errorf("expected component Dashboard, got %s", page.Component)
	}
	if page.URL != "/dashboard" {
		t.Errorf("expected URL /dashboard, got %s", page.URL)
	}
	if page.Version != "1.0" {
		t.Errorf("expected version 1.0, got %s", page.Version)
	}
}

func TestRenderJSON_HasCorrectHeaders(t *testing.T) {
	app, inertia := setupTest()

	app.Get("/test", func(c *fiber.Ctx) error {
		return inertia.Render(c, "Test", fiber.Map{})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Inertia", "true")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		header string
		value  string
	}{
		{"X-Inertia", "true"},
		{"X-Inertia-Version", "1.0"},
		{"Cache-Control", "no-store, no-cache, must-revalidate, private"},
		{"Vary", "X-Inertia"},
		{"Content-Type", "application/json"},
	}
	for _, tc := range tests {
		if got := resp.Header.Get(tc.header); got != tc.value {
			t.Errorf("expected %s: %s, got: %s", tc.header, tc.value, got)
		}
	}
}

func TestRenderJSON_URLIncludesQueryString(t *testing.T) {
	app, inertia := setupTest()

	app.Get("/search", func(c *fiber.Ctx) error {
		return inertia.Render(c, "Search", fiber.Map{})
	})

	req := httptest.NewRequest("GET", "/search?q=golang", nil)
	req.Header.Set("X-Inertia", "true")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}

	page := jsonBody(t, readBody(t, resp))
	if page.URL != "/search?q=golang" {
		t.Errorf("expected URL with query string, got %s", page.URL)
	}
}

func TestRenderJSON_WithNilProps(t *testing.T) {
	app, inertia := setupTest()

	app.Get("/nil", func(c *fiber.Ctx) error {
		return inertia.Render(c, "NilProps", nil)
	})

	req := httptest.NewRequest("GET", "/nil", nil)
	req.Header.Set("X-Inertia", "true")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}

	page := jsonBody(t, readBody(t, resp))
	if page.Component != "NilProps" {
		t.Errorf("expected component NilProps, got %s", page.Component)
	}
	if page.Props == nil {
		t.Error("expected non-nil props (even for nil input)")
	}
}

// ---------------------------------------------------------------------------
// Render — HTML (initial load)
// ---------------------------------------------------------------------------

func TestRenderHTML_ReturnsPageData(t *testing.T) {
	app, inertia := setupTest()

	app.Get("/", func(c *fiber.Ctx) error {
		return inertia.Render(c, "Home", fiber.Map{})
	})

	// No X-Inertia header → initial page load
	req := httptest.NewRequest("GET", "/", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if ct != "text/html; charset=utf-8" {
		t.Errorf("expected text/html, got %s", ct)
	}

	body := readBody(t, resp)
	if len(body) == 0 {
		t.Fatal("expected non-empty HTML body")
	}
}

func TestRenderHTML_ContainsDataPageAttribute(t *testing.T) {
	app, inertia := setupTest()

	app.Get("/", func(c *fiber.Ctx) error {
		return inertia.Render(c, "Welcome", fiber.Map{
			"user": "maulana",
		})
	})

	req := httptest.NewRequest("GET", "/", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}

	body := readBody(t, resp)
	if !contains(string(body), `data-page'`) && !contains(string(body), `data-page=`) {
		t.Error("expected data-page attribute in HTML")
	}
	if !contains(string(body), "Welcome") {
		t.Error("expected component name in HTML")
	}
}

func TestRenderHTML_HasCorrectVaryHeader(t *testing.T) {
	app, inertia := setupTest()

	app.Get("/", func(c *fiber.Ctx) error {
		return inertia.Render(c, "Home", fiber.Map{})
	})

	req := httptest.NewRequest("GET", "/", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}

	if resp.Header.Get("Vary") != "X-Inertia" {
		t.Errorf("expected Vary: X-Inertia, got: %s", resp.Header.Get("Vary"))
	}
}

func TestRenderHTML_WithCustomRenderFunc(t *testing.T) {
	app, inertia := setupTest(func(cfg *Config) {
		cfg.Render = func(c *fiber.Ctx, page *Page) error {
			pageJSON, _ := json.Marshal(page)
			return c.Type("html").SendString("<custom>" + string(pageJSON) + "</custom>")
		}
	})

	app.Get("/", func(c *fiber.Ctx) error {
		return inertia.Render(c, "Custom", fiber.Map{"key": "val"})
	})

	req := httptest.NewRequest("GET", "/", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}

	body := readBody(t, resp)
	if !contains(string(body), "<custom>") || !contains(string(body), "Custom") || !contains(string(body), "</custom>") {
		t.Errorf("expected custom HTML wrapper, got: %s", body)
	}
}

// ---------------------------------------------------------------------------
// Version mismatch
// ---------------------------------------------------------------------------

func TestMiddleware_VersionMatch_PassesThrough(t *testing.T) {
	app, inertia := setupTest()

	app.Get("/", func(c *fiber.Ctx) error {
		return inertia.Render(c, "Home", fiber.Map{})
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Inertia", "true")
	req.Header.Set("X-Inertia-Version", "1.0")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("expected 200 on version match, got %d", resp.StatusCode)
	}
}

func TestMiddleware_VersionMismatch_Returns409(t *testing.T) {
	app, _ := setupTest()

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("should not reach")
	})

	req := httptest.NewRequest("GET", "/dashboard", nil)
	req.Header.Set("X-Inertia", "true")
	req.Header.Set("X-Inertia-Version", "0.9") // different from server "1.0"
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != fiber.StatusConflict {
		t.Errorf("expected 409 on version mismatch, got %d", resp.StatusCode)
	}
}

func TestMiddleware_VersionMismatch_SetsLocation(t *testing.T) {
	app, _ := setupTest()

	app.Get("/dashboard", func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	req := httptest.NewRequest("GET", "/dashboard", nil)
	req.Header.Set("X-Inertia", "true")
	req.Header.Set("X-Inertia-Version", "0.9")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}

	location := resp.Header.Get("X-Inertia-Location")
	if location != "/dashboard" {
		t.Errorf("expected X-Inertia-Location: /dashboard, got: %s", location)
	}
}

func TestMiddleware_EmptyVersion_NoVersionCheck(t *testing.T) {
	app, inertia := setupTest(func(cfg *Config) {
		cfg.Version = "" // Disable version checking
	})

	app.Get("/", func(c *fiber.Ctx) error {
		return inertia.Render(c, "Home", fiber.Map{})
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Inertia", "true")
	req.Header.Set("X-Inertia-Version", "anything")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("expected 200 when version checking is disabled, got %d", resp.StatusCode)
	}
}

func TestMiddleware_NoClientVersion_PassesThrough(t *testing.T) {
	app, inertia := setupTest()

	app.Get("/", func(c *fiber.Ctx) error {
		return inertia.Render(c, "Home", fiber.Map{})
	})

	// Inertia request but no X-Inertia-Version header
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Inertia", "true")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("expected 200 when client sends no version, got %d", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// Shared props
// ---------------------------------------------------------------------------

func TestShare_StaticProp_IncludedInJSON(t *testing.T) {
	app, inertia := setupTest()

	inertia.Share("appName", "MyApp")

	app.Get("/", func(c *fiber.Ctx) error {
		return inertia.Render(c, "Home", fiber.Map{})
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Inertia", "true")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}

	page := jsonBody(t, readBody(t, resp))
	if page.Props["appName"] != "MyApp" {
		t.Errorf("expected shared prop appName=MyApp, got %v", page.Props["appName"])
	}
}

func TestShareFunc_DynamicProp_Included(t *testing.T) {
	app, inertia := setupTest()

	inertia.ShareFunc("requestId", func(c *fiber.Ctx) interface{} {
		return c.Get("X-Request-ID")
	})

	app.Get("/", func(c *fiber.Ctx) error {
		return inertia.Render(c, "Home", fiber.Map{})
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Inertia", "true")
	req.Header.Set("X-Request-ID", "abc-123")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}

	page := jsonBody(t, readBody(t, resp))
	if page.Props["requestId"] != "abc-123" {
		t.Errorf("expected requestId=abc-123, got %v", page.Props["requestId"])
	}
}

func TestShare_PagePropOverridesShared(t *testing.T) {
	app, inertia := setupTest()

	inertia.Share("user", fiber.Map{"role": "guest"})

	app.Get("/", func(c *fiber.Ctx) error {
		return inertia.Render(c, "Home", fiber.Map{
			"user": fiber.Map{"role": "admin"},
		})
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Inertia", "true")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}

	page := jsonBody(t, readBody(t, resp))
	user, ok := page.Props["user"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected user prop to be a map, got %T", page.Props["user"])
	}
	if user["role"] != "admin" {
		t.Errorf("expected page prop to override shared with role=admin, got %v", user["role"])
	}
}

func TestShare_AfterShareFunc_Overwrites(t *testing.T) {
	app, inertia := setupTest()

	inertia.ShareFunc("key", func(c *fiber.Ctx) interface{} {
		return "from-func"
	})
	inertia.Share("key", "from-static") // overwrite

	app.Get("/", func(c *fiber.Ctx) error {
		return inertia.Render(c, "Home", fiber.Map{})
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Inertia", "true")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}

	page := jsonBody(t, readBody(t, resp))
	if page.Props["key"] != "from-static" {
		t.Errorf("expected key=from-static (overwritten), got %v", page.Props["key"])
	}
}

func TestShare_SameKeyReplaces(t *testing.T) {
	app, inertia := setupTest()

	inertia.Share("x", "first")
	inertia.Share("x", "second") // replace

	app.Get("/", func(c *fiber.Ctx) error {
		return inertia.Render(c, "Home", fiber.Map{})
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Inertia", "true")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}

	page := jsonBody(t, readBody(t, resp))
	if page.Props["x"] != "second" {
		t.Errorf("expected x=second, got %v", page.Props["x"])
	}
}

// ---------------------------------------------------------------------------
// Redirects
// ---------------------------------------------------------------------------

func TestRedirect_Returns303(t *testing.T) {
	_, inertia := setupTest()

	app := fiber.New()
	app.Post("/submit", func(c *fiber.Ctx) error {
		return inertia.Redirect(c, "/success")
	})

	req := httptest.NewRequest("POST", "/submit", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != fiber.StatusSeeOther {
		t.Errorf("expected 303, got %d", resp.StatusCode)
	}

	location := resp.Header.Get("Location")
	if location != "/success" {
		t.Errorf("expected Location: /success, got: %s", location)
	}
}

func TestLocation_Returns409WithHeader(t *testing.T) {
	_, inertia := setupTest()

	app := fiber.New()
	app.Get("/leave", func(c *fiber.Ctx) error {
		return inertia.Location(c, "https://example.com")
	})

	req := httptest.NewRequest("GET", "/leave", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != fiber.StatusConflict {
		t.Errorf("expected 409, got %d", resp.StatusCode)
	}

	location := resp.Header.Get("X-Inertia-Location")
	if location != "https://example.com" {
		t.Errorf("expected X-Inertia-Location: https://example.com, got: %s", location)
	}
}

func TestBack_WithReferer(t *testing.T) {
	_, inertia := setupTest()

	app := fiber.New()
	app.Post("/go-back", func(c *fiber.Ctx) error {
		return inertia.Back(c)
	})

	req := httptest.NewRequest("POST", "/go-back", nil)
	req.Header.Set("Referer", "/previous-page")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != fiber.StatusSeeOther {
		t.Errorf("expected 303, got %d", resp.StatusCode)
	}

	location := resp.Header.Get("Location")
	if location != "/previous-page" {
		t.Errorf("expected Location: /previous-page, got: %s", location)
	}
}

func TestBack_WithoutReferer_FallsbackToDefault(t *testing.T) {
	_, inertia := setupTest()

	app := fiber.New()
	app.Post("/go-back", func(c *fiber.Ctx) error {
		return inertia.Back(c, "/fallback")
	})

	req := httptest.NewRequest("POST", "/go-back", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}

	location := resp.Header.Get("Location")
	if location != "/fallback" {
		t.Errorf("expected Location: /fallback, got: %s", location)
	}
}

func TestBack_WithoutRefererNoDefault_FallsbackToRoot(t *testing.T) {
	_, inertia := setupTest()

	app := fiber.New()
	app.Post("/go-back", func(c *fiber.Ctx) error {
		return inertia.Back(c)
	})

	req := httptest.NewRequest("POST", "/go-back", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}

	location := resp.Header.Get("Location")
	if location != "/" {
		t.Errorf("expected Location: /, got: %s", location)
	}
}

// ---------------------------------------------------------------------------
// Partial reloads
// ---------------------------------------------------------------------------

func TestPartialReload_WithPartialData(t *testing.T) {
	app, inertia := setupTest()

	app.Get("/dashboard", func(c *fiber.Ctx) error {
		return inertia.Render(c, "Dashboard", fiber.Map{
			"users": []string{"a", "b"},
			"posts": []string{"p1", "p2"},
		})
	})

	req := httptest.NewRequest("GET", "/dashboard", nil)
	req.Header.Set("X-Inertia", "true")
	req.Header.Set("X-Inertia-Partial-Component", "Dashboard")
	req.Header.Set("X-Inertia-Partial-Data", "users")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}

	page := jsonBody(t, readBody(t, resp))
	if _, hasUsers := page.Props["users"]; !hasUsers {
		t.Error("expected 'users' prop in partial response")
	}
	if _, hasPosts := page.Props["posts"]; hasPosts {
		t.Error("did NOT expect 'posts' prop in partial response with Data=users")
	}
}

func TestPartialReload_WithPartialExcept(t *testing.T) {
	app, inertia := setupTest()

	app.Get("/dashboard", func(c *fiber.Ctx) error {
		return inertia.Render(c, "Dashboard", fiber.Map{
			"users":         []string{"a", "b"},
			"notifications": []string{"n1"},
		})
	})

	req := httptest.NewRequest("GET", "/dashboard", nil)
	req.Header.Set("X-Inertia", "true")
	req.Header.Set("X-Inertia-Partial-Component", "Dashboard")
	req.Header.Set("X-Inertia-Partial-Except", "notifications")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}

	page := jsonBody(t, readBody(t, resp))
	if _, hasUsers := page.Props["users"]; !hasUsers {
		t.Error("expected 'users' prop")
	}
	if _, hasNotif := page.Props["notifications"]; hasNotif {
		t.Error("did NOT expect 'notifications' prop with Except=notifications")
	}
}

func TestPartialReload_WrongComponent_ReturnsAllProps(t *testing.T) {
	app, inertia := setupTest()

	app.Get("/dashboard", func(c *fiber.Ctx) error {
		return inertia.Render(c, "Dashboard", fiber.Map{
			"a": 1,
			"b": 2,
		})
	})

	req := httptest.NewRequest("GET", "/dashboard", nil)
	req.Header.Set("X-Inertia", "true")
	req.Header.Set("X-Inertia-Partial-Component", "OtherPage") // different component!
	req.Header.Set("X-Inertia-Partial-Data", "a")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}

	page := jsonBody(t, readBody(t, resp))
	if page.Props["a"] == nil || page.Props["b"] == nil {
		t.Error("expected ALL props when partial component doesn't match")
	}
}

func TestPartialReload_ExceptTakesPrecedenceOverData(t *testing.T) {
	app, inertia := setupTest()

	app.Get("/dashboard", func(c *fiber.Ctx) error {
		return inertia.Render(c, "Dashboard", fiber.Map{
			"a": 1,
			"b": 2,
			"c": 3,
		})
	})

	req := httptest.NewRequest("GET", "/dashboard", nil)
	req.Header.Set("X-Inertia", "true")
	req.Header.Set("X-Inertia-Partial-Component", "Dashboard")
	req.Header.Set("X-Inertia-Partial-Data", "a,b")
	req.Header.Set("X-Inertia-Partial-Except", "b") // except takes precedence
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}

	page := jsonBody(t, readBody(t, resp))
	if page.Props["a"] == nil {
		t.Error("expected 'a' prop")
	}
	if page.Props["b"] != nil {
		t.Error("did NOT expect 'b' prop (except takes precedence)")
	}
}

// ---------------------------------------------------------------------------
// Page object fields
// ---------------------------------------------------------------------------

func TestPageObject_MergeProps(t *testing.T) {
	app, _ := setupTest()

	app.Get("/posts", func(c *fiber.Ctx) error {
		// Use custom page for full control
		page := &Page{
			Component:  "Posts",
			Props:      fiber.Map{"posts": []int{1, 2, 3}},
			URL:        string(c.Request().URI().RequestURI()),
			Version:    "1.0",
			MergeProps: []string{"posts"},
		}

		// We need to bypass Render() for full page object control
		c.Set("X-Inertia", "true")
		c.Set("X-Inertia-Version", "1.0")
		c.Set(fiber.HeaderCacheControl, "no-store, no-cache, must-revalidate, private")
		c.Set(fiber.HeaderVary, "X-Inertia")
		c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
		return c.JSON(page)
	})

	req := httptest.NewRequest("GET", "/posts", nil)
	req.Header.Set("X-Inertia", "true")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}

	page := jsonBody(t, readBody(t, resp))
	if len(page.MergeProps) != 1 || page.MergeProps[0] != "posts" {
		t.Errorf("expected MergeProps=[posts], got %v", page.MergeProps)
	}
}

func TestPageObject_EncryptHistory(t *testing.T) {
	app := fiber.New()
	app.Get("/secure", func(c *fiber.Ctx) error {
		page := &Page{
			Component:      "Secure",
			Props:          fiber.Map{},
			URL:            c.OriginalURL(),
			Version:        "1.0",
			EncryptHistory: BoolPtr(true),
		}
		c.Set("X-Inertia", "true")
		c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
		return c.JSON(page)
	})

	req := httptest.NewRequest("GET", "/secure", nil)
	req.Header.Set("X-Inertia", "true")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}

	page := jsonBody(t, readBody(t, resp))
	if page.EncryptHistory == nil || *page.EncryptHistory != true {
		t.Error("expected EncryptHistory=true")
	}
}

func TestPageObject_ClearHistory(t *testing.T) {
	app := fiber.New()
	app.Get("/clear", func(c *fiber.Ctx) error {
		page := &Page{
			Component:    "Clear",
			Props:        fiber.Map{},
			URL:          c.OriginalURL(),
			Version:      "1.0",
			ClearHistory: BoolPtr(true),
		}
		c.Set("X-Inertia", "true")
		c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
		return c.JSON(page)
	})

	req := httptest.NewRequest("GET", "/clear", nil)
	req.Header.Set("X-Inertia", "true")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}

	page := jsonBody(t, readBody(t, resp))
	if page.ClearHistory == nil || *page.ClearHistory != true {
		t.Error("expected ClearHistory=true")
	}
}

// ---------------------------------------------------------------------------
// Edge cases
// ---------------------------------------------------------------------------

func TestRender_NonInertiaRequest_ReturnsHTML(t *testing.T) {
	app, inertia := setupTest()

	app.Get("/page", func(c *fiber.Ctx) error {
		return inertia.Render(c, "Page", fiber.Map{})
	})

	// No Inertia headers
	req := httptest.NewRequest("GET", "/page", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}

	ct := resp.Header.Get("Content-Type")
	if !contains(ct, "text/html") {
		t.Errorf("expected text/html, got %s", ct)
	}
}

func TestRender_InertiaRequest_ReturnsJSON(t *testing.T) {
	app, inertia := setupTest()

	app.Get("/page", func(c *fiber.Ctx) error {
		return inertia.Render(c, "Page", fiber.Map{})
	})

	req := httptest.NewRequest("GET", "/page", nil)
	req.Header.Set("X-Inertia", "true")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}

	ct := resp.Header.Get("Content-Type")
	if !contains(ct, "application/json") {
		t.Errorf("expected application/json, got %s", ct)
	}
}

func TestVersionFromFile_FileNotFound(t *testing.T) {
	v, err := VersionFromFile("/tmp/nonexistent-file-xyz.txt")
	if err != nil {
		t.Errorf("expected no error for missing file, got: %v", err)
	}
	if v != "" {
		t.Errorf("expected empty version, got: %s", v)
	}
}

func TestExtractTitle_FromTitle(t *testing.T) {
	props := fiber.Map{"title": "My App"}
	title := extractTitle(props)
	if title != "My App" {
		t.Errorf("expected 'My App', got '%s'", title)
	}
}

func TestExtractTitle_FromUnderscoreTitle(t *testing.T) {
	props := fiber.Map{"_title": "Dashboard"}
	title := extractTitle(props)
	if title != "Dashboard" {
		t.Errorf("expected 'Dashboard', got '%s'", title)
	}
}

func TestExtractTitle_UnderscoreTitleTakesPrecedence(t *testing.T) {
	props := fiber.Map{
		"_title": "Dashboard",
		"title":  "My App",
	}
	title := extractTitle(props)
	if title != "Dashboard" {
		t.Errorf("expected '_title' to take precedence, got '%s'", title)
	}
}

func TestExtractTitle_EmptyProps(t *testing.T) {
	title := extractTitle(fiber.Map{})
	if title != "Inertia" {
		t.Errorf("expected default 'Inertia', got '%s'", title)
	}
}

func TestExtractTitle_NilProps(t *testing.T) {
	title := extractTitle(nil)
	if title != "Inertia" {
		t.Errorf("expected default 'Inertia', got '%s'", title)
	}
}

func TestForceVersionCheck_Match_NoConflict(t *testing.T) {
	_, inertia := setupTest()

	app := fiber.New()
	app.Get("/", func(c *fiber.Ctx) error {
		conflict, err := inertia.ForceVersionCheck(c)
		if conflict {
			return err
		}
		return c.SendString("OK")
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Inertia", "true")
	req.Header.Set("X-Inertia-Version", "1.0")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("expected 200 on version match, got %d", resp.StatusCode)
	}
}

func TestForceVersionCheck_Mismatch_409(t *testing.T) {
	_, inertia := setupTest()

	app := fiber.New()
	app.Get("/check", func(c *fiber.Ctx) error {
		conflict, err := inertia.ForceVersionCheck(c)
		if conflict {
			return err
		}
		return c.SendString("OK")
	})

	req := httptest.NewRequest("GET", "/check", nil)
	req.Header.Set("X-Inertia", "true")
	req.Header.Set("X-Inertia-Version", "0.5")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != fiber.StatusConflict {
		t.Errorf("expected 409 on version mismatch, got %d", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// Integration: shared props + partial reload combined
// ---------------------------------------------------------------------------

func TestSharedProps_WithPartialReload(t *testing.T) {
	app, inertia := setupTest()

	inertia.Share("appName", "MyApp")

	app.Get("/dashboard", func(c *fiber.Ctx) error {
		return inertia.Render(c, "Dashboard", fiber.Map{
			"users": []string{"a"},
			"stats": fiber.Map{"visits": 100},
		})
	})

	// Partial reload with only "users"
	req := httptest.NewRequest("GET", "/dashboard", nil)
	req.Header.Set("X-Inertia", "true")
	req.Header.Set("X-Inertia-Partial-Component", "Dashboard")
	req.Header.Set("X-Inertia-Partial-Data", "users")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}

	page := jsonBody(t, readBody(t, resp))
	if page.Props["appName"] != "MyApp" {
		t.Errorf("expected shared prop 'appName' in partial response, got %v", page.Props["appName"])
	}
	if _, ok := page.Props["users"]; !ok {
		t.Error("expected 'users' in partial response")
	}
	if _, ok := page.Props["stats"]; ok {
		t.Error("did NOT expect 'stats' in partial response")
	}
}

// ---------------------------------------------------------------------------
// Non-Inertia requests should pass through middleware unaffected
// ---------------------------------------------------------------------------

func TestMiddleware_NonInertiaRequest_PassesThrough(t *testing.T) {
	app, _ := setupTest()

	app.Get("/api/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/api/health", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("expected 200 for non-Inertia request, got %d", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && searchString(s, substr)
}

// searchString is a simple substring search to avoid importing strings.
func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
