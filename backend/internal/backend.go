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
	BuildDir    string
}

// NewBackend creates a new backend application. You must call Start() to start the application.
func NewBackend(username, countryCode, buildDir string) *Backend {
	backend := &Backend{
		PocketBase:  pocketbase.New(),
		Username:    username,
		CountryCode: countryCode,
		BuildDir:    buildDir,
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
		go backend.runJobs()

		return se.Next()
	})

	if err := backend.PocketBase.Cron().Add("sync-bgg", "@daily", backend.runJobs); err != nil {
		backend.Logger().
			With("job", "sync-bgg").
			Error("Failed to add cron job", "error", err)
	}

	if err := backend.PocketBase.Cron().Add("build-frontend", "@yearly", func() {
		if err := backend.buildFrontend(); err != nil {
			backend.Logger().Error("Failed to build frontend", "error", err)
		}
	}); err != nil {
		backend.Logger().
			With("job", "sync-bgg").
			Error("Failed to add cron job", "error", err)
	}

	return backend
}

func (b *Backend) runJobs() {
	triggerBGGRebuild := b.syncBGGWishlist(b.Username)
	triggerBGORebuild := b.syncBGOPrices()

	if triggerBGGRebuild || triggerBGORebuild {
		b.Logger().
			With("bgg_rebuild", triggerBGGRebuild).
			With("bgo_rebuild", triggerBGORebuild).
			Info("Triggering frontend rebuild")
		if err := b.buildFrontend(); err != nil {
			b.Logger().Error("Failed to build frontend", "error", err)
		}
	}
}

// Start starts the backend application.
func (b *Backend) Start() error {
	return b.PocketBase.Start()
}
