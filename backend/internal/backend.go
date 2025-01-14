package internal

import (
	"net/http"
	"os"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/plugins/migratecmd"

	_ "github.com/sneat/wishlist/backend/migrations"
)

// Backend application.
type Backend struct {
	*pocketbase.PocketBase
	Username    string
	CountryCode string
}

// NewBackend creates a new backend application. You must call Start() to start the application.
func NewBackend(username, countryCode string) *Backend {
	backend := &Backend{
		PocketBase:  pocketbase.New(),
		Username:    username,
		CountryCode: countryCode,
	}

	migratecmd.MustRegister(backend, backend.RootCmd, migratecmd.Config{
		Automigrate: false,
	})

	backend.PocketBase.OnServe().BindFunc(func(se *core.ServeEvent) error {
		// serves static files from the provided public dir (if exists)
		se.Router.GET("/{path...}", apis.Static(os.DirFS("./pb_public"), false))

		se.Router.GET("/api/v1/bgg-wishlist", func(e *core.RequestEvent) error {
			records, err := backend.FindRecordsByFilter(
				"items",
				"deleted = false",
				"", // no sort
				0,  // no limit
				0,  // no offset
			)
			if err != nil {
				backend.Logger().Error("Failed to fetch BGG wishlist items", "error", err)
				return e.Error(
					http.StatusInternalServerError,
					"",
					"Failed to fetch BGG wishlist items",
				)
			}

			return e.JSON(http.StatusOK, records)
		})

		// Trigger a sync on initial load
		go func(b *Backend) {
			b.syncBGGWishlist(backend.Username)
			b.syncBGOPrices()
		}(backend)

		return se.Next()
	})

	backend.PocketBase.Cron().MustAdd("sync-bgg", "0 * * * *", func() {
		backend.syncBGGWishlist(backend.Username)
		backend.syncBGOPrices()
	})

	return backend
}

// Start starts the backend application.
func (b *Backend) Start() error {
	return b.PocketBase.Start()
}
