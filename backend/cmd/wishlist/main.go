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

	username := os.Getenv("USERNAME")
	countryCode := os.Getenv("COUNTRY_CODE")
	buildDir := os.Getenv("DIR")
	backend := internal.NewBackend(username, countryCode, buildDir)

	if err := backend.Start(); err != nil {
		log.Fatal(err)
	}
}
