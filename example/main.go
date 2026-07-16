package main

import (
	"log"

	"github.com/gofiber/fiber/v2"
	fiberinertia "github.com/maulanashalihin/fiber-inertia"
)

func main() {
	app := fiber.New(fiber.Config{
		AppName: "Fiber Inertia Example",
	})

	// Create Inertia adapter with version "1.0"
	inertia := fiberinertia.New(fiberinertia.Config{
		Version: "1.0",
	})

	// Middleware: asset version checking
	app.Use(inertia.Middleware())

	// Shared props: available on every page
	inertia.Share("appName", "Fiber Inertia")
	inertia.ShareFunc("year", func(c *fiber.Ctx) interface{} {
		return "2026"
	})

	// Home page
	app.Get("/", func(c *fiber.Ctx) error {
		return inertia.Render(c, "Home", fiber.Map{
			"title": "Home",
			"body":  "Welcome to Fiber Inertia!",
		})
	})

	// Dashboard (requires login redirect example)
	app.Get("/dashboard", func(c *fiber.Ctx) error {
		return inertia.Render(c, "Dashboard", fiber.Map{
			"title": "Dashboard",
			"user": fiber.Map{
				"name":  "Maulana",
				"email": "maulana@example.com",
			},
		})
	})

	// Form submit → redirect
	app.Post("/login", func(c *fiber.Ctx) error {
		// ... authenticate ...
		return inertia.Redirect(c, "/dashboard")
	})

	// Logout → external redirect
	app.Post("/logout", func(c *fiber.Ctx) error {
		// ... clear session ...
		return inertia.Location(c, "https://google.com")
	})

	// Login page (with custom render for initial load)
	app.Get("/login", func(c *fiber.Ctx) error {
		return inertia.Render(c, "Login", fiber.Map{
			"title": "Login",
		})
	})

	log.Println("Server listening on :3000")
	log.Fatal(app.Listen(":3000"))
}
