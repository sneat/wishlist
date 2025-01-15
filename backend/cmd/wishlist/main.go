package main

import (
	"log"
	"os"

	"github.com/sneat/wishlist/backend/internal"
)

func main() {
	username := os.Getenv("USERNAME")
	countryCode := os.Getenv("COUNTRY_CODE")
	buildDir := os.Getenv("DIR")
	backend := internal.NewBackend(username, countryCode, buildDir)

	if err := backend.Start(); err != nil {
		log.Fatal(err)
	}
}
