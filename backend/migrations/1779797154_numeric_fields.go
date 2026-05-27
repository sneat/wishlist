// Numeric schema fields migration.
//
// BEFORE RUNNING THIS MIGRATION IN PRODUCTION: take a manual filesystem snapshot —
// e.g. `cp -r pb_data pb_data.bak` — as belt-and-suspenders insurance. The migration
// itself writes an automatic snapshot to pb_data/backups/pre-numeric-migration-*.zip
// and runs the schema swap + backfill inside a single SQLite transaction, but a manual
// copy makes recovery trivial if anything wedges PocketBase itself.

package migrations

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(numericFieldsUp, numericFieldsDown)
}

// fields converted to NumberField{OnlyInt: true}.
var numericIntFields = []string{
	"price",
	"year_published",
	"priority",
	"playing_time",
	"bgg_rank",
}

const (
	numericFloatField  = "rating"
	oldBestPlayerCount = "best_player_count_numbers"
	newBestPlayerCount = "best_player_count_number"
)

func numericFieldsUp(app core.App) error {
	backupName := fmt.Sprintf("pre-numeric-migration-%d.zip", time.Now().Unix())
	if err := app.CreateBackup(context.Background(), backupName); err != nil {
		return fmt.Errorf("create backup before migration: %w", err)
	}

	return app.RunInTransaction(func(txApp core.App) error {
		collection, err := txApp.FindCollectionByNameOrId("items")
		if err != nil {
			return err
		}

		records, err := txApp.FindRecordsByFilter("items", "", "", 0, 0)
		if err != nil {
			return fmt.Errorf("snapshot items records: %w", err)
		}

		type snapshot struct {
			ints             map[string]string
			floatRating      string
			bestPlayerCountN string
		}
		captured := make(map[string]snapshot, len(records))
		for _, r := range records {
			s := snapshot{ints: make(map[string]string, len(numericIntFields))}
			for _, f := range numericIntFields {
				s.ints[f] = r.GetString(f)
			}
			s.floatRating = r.GetString(numericFloatField)
			s.bestPlayerCountN = r.GetString(oldBestPlayerCount)
			captured[r.Id] = s
		}

		for _, name := range numericIntFields {
			collection.Fields.RemoveByName(name)
			collection.Fields.Add(&core.NumberField{Name: name, OnlyInt: true})
		}
		collection.Fields.RemoveByName(numericFloatField)
		collection.Fields.Add(&core.NumberField{Name: numericFloatField})

		collection.Fields.RemoveByName(oldBestPlayerCount)
		collection.Fields.Add(&core.NumberField{Name: newBestPlayerCount, OnlyInt: true})

		if err := txApp.Save(collection); err != nil {
			return fmt.Errorf("save converted schema: %w", err)
		}

		// Re-fetch records against the new schema. The pre-swap records still carry the
		// dropped best_player_count_numbers column and would fail to save.
		freshRecords, err := txApp.FindRecordsByFilter("items", "", "", 0, 0)
		if err != nil {
			return fmt.Errorf("re-fetch items after schema swap: %w", err)
		}

		for _, r := range freshRecords {
			snap, ok := captured[r.Id]
			if !ok {
				continue
			}
			for _, f := range numericIntFields {
				r.Set(f, parseIntOrZeroLocal(snap.ints[f]))
			}
			r.Set(numericFloatField, parseFloatOrZeroLocal(snap.floatRating))
			r.Set(newBestPlayerCount, parseIntOrZeroLocal(snap.bestPlayerCountN))

			if err := txApp.Save(r); err != nil {
				return fmt.Errorf("backfill record %s: %w", r.Id, err)
			}
		}

		return nil
	})
}

func numericFieldsDown(app core.App) error {
	return app.RunInTransaction(func(txApp core.App) error {
		collection, err := txApp.FindCollectionByNameOrId("items")
		if err != nil {
			return err
		}

		records, err := txApp.FindRecordsByFilter("items", "", "", 0, 0)
		if err != nil {
			return err
		}

		type snapshot struct {
			ints             map[string]int
			floatRating      float64
			bestPlayerCountN int
		}
		captured := make(map[string]snapshot, len(records))
		for _, r := range records {
			s := snapshot{ints: make(map[string]int, len(numericIntFields))}
			for _, f := range numericIntFields {
				s.ints[f] = r.GetInt(f)
			}
			s.floatRating = r.GetFloat(numericFloatField)
			s.bestPlayerCountN = r.GetInt(newBestPlayerCount)
			captured[r.Id] = s
		}

		for _, name := range numericIntFields {
			collection.Fields.RemoveByName(name)
			collection.Fields.Add(&core.TextField{Name: name})
		}
		collection.Fields.RemoveByName(numericFloatField)
		collection.Fields.Add(&core.TextField{Name: numericFloatField})

		collection.Fields.RemoveByName(newBestPlayerCount)
		collection.Fields.Add(&core.TextField{Name: oldBestPlayerCount})

		if err := txApp.Save(collection); err != nil {
			return err
		}

		freshRecords, err := txApp.FindRecordsByFilter("items", "", "", 0, 0)
		if err != nil {
			return err
		}

		for _, r := range freshRecords {
			snap, ok := captured[r.Id]
			if !ok {
				continue
			}
			for _, f := range numericIntFields {
				if v := snap.ints[f]; v != 0 {
					r.Set(f, strconv.Itoa(v))
				} else {
					r.Set(f, "")
				}
			}
			if snap.floatRating != 0 {
				r.Set(numericFloatField, fmt.Sprintf("%.2f", snap.floatRating))
			} else {
				r.Set(numericFloatField, "")
			}
			if snap.bestPlayerCountN != 0 {
				r.Set(oldBestPlayerCount, strconv.Itoa(snap.bestPlayerCountN))
			} else {
				r.Set(oldBestPlayerCount, "")
			}

			if err := txApp.Save(r); err != nil {
				return err
			}
		}

		return nil
	})
}

// parseIntOrZeroLocal mirrors internal.parseIntOrZero. The migrations package cannot import
// internal without an import cycle through main, so the small helpers are duplicated here.
func parseIntOrZeroLocal(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0
	}
	return n
}

func parseFloatOrZeroLocal(s string) float64 {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return f
}
