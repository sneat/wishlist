package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		if err := initialCollectionUp(app); err != nil {
			return err
		}

		return nil
	}, func(app core.App) error {
		if err := initialCollectionDown(app); err != nil {
			return err
		}

		return nil
	})
}

func initialCollectionUp(app core.App) error {
	// Delete the existing users collection to prevent signup
	usersCollection, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		return err
	}

	if err := app.Delete(usersCollection); err != nil {
		return err
	}

	// Create the items collection
	itemsCollection := core.NewBaseCollection("items")

	// Add Fields
	itemsCollection.Fields.Add(&core.TextField{
		Name:        "name",
		Presentable: true,
	})
	itemsCollection.Fields.Add(&core.TextField{
		Name: "year_published",
	})
	itemsCollection.Fields.Add(&core.TextField{
		Name: "price",
	})
	itemsCollection.Fields.Add(&core.TextField{
		Name: "rating",
	})
	itemsCollection.Fields.Add(&core.TextField{
		Name: "players",
	})
	itemsCollection.Fields.Add(&core.TextField{
		Name: "priority",
	})
	itemsCollection.Fields.Add(&core.TextField{
		Name: "playing_time",
	})
	itemsCollection.Fields.Add(&core.TextField{
		Name: "bgg_id",
	})
	itemsCollection.Fields.Add(&core.TextField{
		Name: "bgo_id",
	})
	itemsCollection.Fields.Add(&core.TextField{
		Name: "thumbnail",
	})
	itemsCollection.Fields.Add(&core.TextField{
		Name: "image",
	})
	itemsCollection.Fields.Add(&core.BoolField{
		Name:   "deleted",
		Hidden: true,
	})

	// Add indexes
	itemsCollection.AddIndex("idx_bgg_id", false, "bgg_id", "")
	itemsCollection.AddIndex("idx_bgo_id", false, "bgo_id", "")
	itemsCollection.AddIndex("idx_deleted", false, "deleted", "")
	itemsCollection.AddIndex("idx_name", false, "name", "")

	return app.Save(itemsCollection)
}

func initialCollectionDown(app core.App) error {
	collection, err := app.FindCollectionByNameOrId("items")
	if err != nil {
		return err
	}

	return app.Delete(collection)
}
