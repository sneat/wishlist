package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("items")
		if err != nil {
			return err
		}

		collection.Fields.Add(&core.JSONField{
			Name: "price_history",
		})

		return app.Save(collection)
	}, func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("items")
		if err != nil {
			return err
		}

		collection.Fields.RemoveByName("price_history")

		return app.Save(collection)
	})
}
