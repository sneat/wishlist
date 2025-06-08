package migrations

import (
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
	"time"
)

func init() {
	m.Register(func(app core.App) error {
		itemsCollection, err := app.FindCollectionByNameOrId("items")
		if err != nil {
			return err
		}

		itemsCollection.Fields.Add(&core.BoolField{
			Name: "is_owned",
		})
		itemsCollection.Fields.Add(&core.DateField{
			Name: "last_modified",
		})
		itemsCollection.Fields.Add(&core.AutodateField{
			Name:     "created_at",
			OnCreate: true,
		})

		if err = app.Save(itemsCollection); err != nil {
			return err
		}

		oneMonthAgo := time.Now().AddDate(0, -1, 0).
			UTC().Format("2006-01-02 15:04:05.000Z")
		if _, err = app.DB().NewQuery(
			"UPDATE items SET created_at = {:date} WHERE created_at = ''",
		).
			Bind(dbx.Params{
				"date": oneMonthAgo,
			}).Execute(); err != nil {
			return err
		}

		return nil
	}, func(app core.App) error {
		itemsCollection, err := app.FindCollectionByNameOrId("items")
		if err != nil {
			return err
		}

		itemsCollection.Fields.RemoveByName("is_owned")
		itemsCollection.Fields.RemoveByName("created_at")

		return app.Save(itemsCollection)
	})
}
