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

		// Add description field (long text for game description)
		itemsCollection.Fields.Add(&core.TextField{
			Name: "description",
		})

		// Add minimum age field
		itemsCollection.Fields.Add(&core.NumberField{
			Name: "minage",
		})

		// Add best player count recommendation (display text from poll-summary)
		itemsCollection.Fields.Add(&core.TextField{
			Name: "best_player_count",
		})

		// Add best player count number (single number with most Best votes, for filtering)
		itemsCollection.Fields.Add(&core.TextField{
			Name: "best_player_count_numbers",
		})

		// Add categories as JSON field
		itemsCollection.Fields.Add(&core.JSONField{
			Name: "categories",
		})

		// Add mechanics as JSON field
		itemsCollection.Fields.Add(&core.JSONField{
			Name: "mechanics",
		})

		// Add BGG rank
		itemsCollection.Fields.Add(&core.TextField{
			Name: "bgg_rank",
		})

		// Add details last fetched timestamp
		itemsCollection.Fields.Add(&core.DateField{
			Name: "details_last_fetched",
		})

		// Add index on details_last_fetched for efficient queries
		itemsCollection.AddIndex("idx_details_last_fetched", false, "details_last_fetched", "")

		return app.Save(itemsCollection)
	}, func(app core.App) error {
		itemsCollection, err := app.FindCollectionByNameOrId("items")
		if err != nil {
			return err
		}

		// Remove fields in reverse order
		itemsCollection.RemoveIndex("idx_details_last_fetched")
		itemsCollection.Fields.RemoveByName("details_last_fetched")
		itemsCollection.Fields.RemoveByName("bgg_rank")
		itemsCollection.Fields.RemoveByName("mechanics")
		itemsCollection.Fields.RemoveByName("categories")
		itemsCollection.Fields.RemoveByName("best_player_count_numbers")
		itemsCollection.Fields.RemoveByName("best_player_count")
		itemsCollection.Fields.RemoveByName("minage")
		itemsCollection.Fields.RemoveByName("description")

		return app.Save(itemsCollection)
	})
}
