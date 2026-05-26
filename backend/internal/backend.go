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

	username     string
	password     string
	countryCode  string
	buildDir     string
	bggAuthToken string
}

// BackendOption configures Backend.
type BackendOption func(*Backend)

// WithUsername sets the backend username.
func WithUsername(username string) BackendOption {
	return func(b *Backend) {
		b.username = username
	}
}

// WithPassword sets the backend password.
func WithPassword(password string) BackendOption {
	return func(b *Backend) {
		b.password = password
	}
}

// WithCountryCode sets the backend country code.
func WithCountryCode(countryCode string) BackendOption {
	return func(b *Backend) {
		b.countryCode = countryCode
	}
}

// WithBuildDir sets the frontend build directory.
func WithBuildDir(buildDir string) BackendOption {
	return func(b *Backend) {
		b.buildDir = buildDir
	}
}

// WithBGGAuthToken sets the auth token for BGG requests.
func WithBGGAuthToken(token string) BackendOption {
	return func(b *Backend) {
		b.bggAuthToken = token
	}
}

// NewBackend creates a new backend application. You must call Start() to start the application.
func NewBackend(opts ...BackendOption) *Backend {
	backend := &Backend{
		PocketBase: pocketbase.New(),
	}

	for _, opt := range opts {
		opt(backend)
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

	if err := backend.PocketBase.Cron().Add("refresh-all-details", "@yearly", func() {
		backend.Logger().Info("Starting yearly full refresh of all thing details")
		if triggerRebuild, err := backend.fetchThingDetailsForItems(true, 0); err != nil {
			backend.Logger().Error("Failed to refresh all thing details", "error", err)
		} else if triggerRebuild {
			if err = backend.buildFrontend(buildModeForceReinstall); err != nil {
				backend.Logger().Error("Failed to build frontend", "error", err)
			}
		}
	}); err != nil {
		backend.Logger().
			With("job", "refresh-all-details").
			Error("Failed to add cron job", "error", err)
	}

	if err := backend.PocketBase.Cron().Add("build-frontend", "@yearly", func() {
		if err := backend.buildFrontend(buildModeForceReinstall); err != nil {
			backend.Logger().Error("Failed to build frontend", "error", err)
		}
	}); err != nil {
		backend.Logger().
			With("job", "build-frontend").
			Error("Failed to add cron job", "error", err)
	}

	return backend
}

func (b *Backend) runJobs() {
	triggerBGGRebuild := b.syncBGGWishlist()
	triggerBGORebuild := b.syncBGOPrices()

	// Fetch thing details for up to 50 items that need it
	triggerDetailsRebuild := false
	if detailsRebuild, err := b.fetchThingDetailsForItems(false, 50); err != nil {
		b.Logger().Error("Failed to fetch thing details", "error", err)
	} else {
		triggerDetailsRebuild = detailsRebuild
	}

	if triggerBGGRebuild || triggerBGORebuild || triggerDetailsRebuild {
		b.Logger().
			With("bgg_rebuild", triggerBGGRebuild).
			With("bgo_rebuild", triggerBGORebuild).
			With("details_rebuild", triggerDetailsRebuild).
			Info("Triggering frontend rebuild")
		if err := b.buildFrontend(buildModeCached); err != nil {
			b.Logger().Error("Failed to build frontend", "error", err)
		}
	}
}

// Start starts the backend application.
func (b *Backend) Start() error {
	return b.PocketBase.Start()
}
