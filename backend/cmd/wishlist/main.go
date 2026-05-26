package main

import (
	"fmt"
	"log"
	"os"

	"github.com/outrigdev/outrig"
	"github.com/sneat/wishlist/backend/internal"
)

func main() {
	if os.Getenv("ENV") == "development" {
		fmt.Println("Running in development mode...")

		cfg := outrig.DefaultConfig()
		cfg.ConnectOnInit = true
		cfg.Quiet = false
		if _, err := outrig.Init("wishlist", cfg); err != nil {
			fmt.Println("Error initializing outrig:", err)
		}
	}

	username := os.Getenv("BGG_USERNAME")
	password := os.Getenv("BGG_PASSWORD")
	countryCode := os.Getenv("COUNTRY_CODE")
	buildDir := os.Getenv("DIR")
	bggAuthToken := os.Getenv("BGG_AUTH_TOKEN")

	backend := internal.NewBackend(
		internal.WithUsername(username),
		internal.WithPassword(password),
		internal.WithCountryCode(countryCode),
		internal.WithBuildDir(buildDir),
		internal.WithBGGAuthToken(bggAuthToken),
	)

	if err := backend.Start(); err != nil {
		log.Fatal(err)
	}
}
