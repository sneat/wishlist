package main

import (
	"log"
	"os"

	"github.com/sneat/wishlist/backend/internal"
)

func main() {
	username := os.Getenv("USERNAME")
	countryCode := os.Getenv("COUNTRY_CODE")
	backend := internal.NewBackend(username, countryCode)

	if err := backend.Start(); err != nil {
		log.Fatal(err)
	}
}
