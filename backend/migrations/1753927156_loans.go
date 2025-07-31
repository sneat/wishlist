package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		itemsCollection, err := app.FindCollectionByNameOrId("items")
		if err != nil {
			return err
		}

		itemsCollection.Fields.Add(&core.TextField{
			Name: "loaned_to",
		})
		itemsCollection.AddIndex("idx_loaned_to", false, "loaned_to", "")

		return app.Save(itemsCollection)
	}, func(app core.App) error {
		itemsCollection, err := app.FindCollectionByNameOrId("items")
		if err != nil {
			return err
		}

		itemsCollection.RemoveIndex("idx_loaned_to")
		itemsCollection.Fields.RemoveByName("loaned_to")

		return app.Save(itemsCollection)
	})
}
