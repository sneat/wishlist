package internal

import (
	"database/sql"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

func (b *Backend) syncBGGWishlist(username string) bool {
	var triggerRebuild bool
	if username == "" {
		b.Logger().Error("No BGG username provided.")
		return triggerRebuild
	}

	start := time.Now()
	b.Logger().Info("Syncing BGG wishlist items.")

	items, err := b.FetchBGGWishlistItems(username)
	if err != nil {
		b.Logger().Error("Failed to fetch BGG wishlist items", "error", err)
		return triggerRebuild
	}

	b.Logger().With("items", len(items)).Info("Fetched BGG wishlist items.")

	triggerRebuild, err = b.processBGGItems(items)
	if err != nil {
		b.Logger().Error("Failed to process BGG items", "error", err)
		return triggerRebuild
	}

	b.Logger().
		With("duration", time.Since(start).String()).
		With("count", len(items)).
		Info("Syncing BGG wishlist items completed.")

	return triggerRebuild
}

// processBGGItems processes the BGG items and updates the database.
// It returns true if the frontend should be rebuilt.
func (b *Backend) processBGGItems(items []BGGItem) (bool, error) {
	var triggerRebuild bool
	collection, err := b.FindCollectionByNameOrId("items")
	if err != nil {
		return triggerRebuild, err
	}

	// Track the items that are processed so we can delete the ones that are no longer in the wishlist
	processedItems := make(map[string]bool)
	newItems := make([]*core.Record, 0)

	for _, item := range items {
		processedItems[item.ObjectID] = true
		b.Logger().Debug(fmt.Sprintf("Processing %s (%s)", item.Name, item.ObjectID))

		if err = b.RunInTransaction(func(txApp core.App) error {
			record, err := b.FindFirstRecordByData("items", "bgg_id", item.ObjectID)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				b.Logger().Warn(fmt.Sprintf("Failed to find record %s (%s)", item.Name, item.ObjectID), "error", err)
			}

			if record == nil {
				record = core.NewRecord(collection)
				newItems = append(newItems, record)
			}

			// Parse the player count as integers
			minPlayers, err := strconv.Atoi(item.Stats.MinPlayers)
			if err != nil {
				minPlayers = 0
			}
			maxPlayers, err := strconv.Atoi(item.Stats.MaxPlayers)
			if err != nil {
				maxPlayers = 0
			}

			players := ""
			if minPlayers != 0 && maxPlayers != 0 {
				players = fmt.Sprintf("%d-%d", minPlayers, maxPlayers)
			} else if minPlayers != 0 {
				players = fmt.Sprintf("%d+", minPlayers)
			} else if maxPlayers != 0 {
				players = fmt.Sprintf("Up to %d", maxPlayers)
			}

			// Parse the rating and round to 2 decimal places
			rating := item.Stats.Rating.Average.Value
			if rating != "" {
				ratingValue, err := strconv.ParseFloat(rating, 64)
				if err != nil {
					rating = ""
				} else {
					rating = fmt.Sprintf("%.2f", ratingValue)
				}
			}

			record.Set("name", item.Name)
			record.Set("thumbnail", item.Thumbnail)
			record.Set("image", item.Image)
			record.Set("year_published", item.YearPublished)
			record.Set("players", players)
			record.Set("rating", rating)
			record.Set("priority", item.Status.WishlistPriority)
			record.Set("playing_time", item.Stats.PlayingTime)
			record.Set("bgg_id", item.ObjectID)
			record.Set("deleted", false)

			if err = txApp.Save(record); err != nil {
				return err
			}

			return nil
		}); err != nil {
			b.Logger().Error(fmt.Sprintf("Failed to process %s", item.Name), "error", err)
		}
	}

	for _, record := range newItems {
		b.Logger().Info(
			fmt.Sprintf(
				"New item added %s (%s) - attempting to automatch to Board Game Oracle",
				record.Get("name"),
				record.Get("bgg_id"),
			),
		)
		if err = b.autoPopulateBGOId(record); err != nil {
			b.Logger().Error(
				fmt.Sprintf(
					"Failed to auto-populate BGO ID for %s (%s)",
					record.Get("name"),
					record.Get("bgg_id"),
				),
				"error",
				err,
			)
		}
	}

	if len(newItems) > 0 {
		triggerRebuild = true
	}

	ids := make([]string, 0, len(processedItems))
	for id := range processedItems {
		ids = append(ids, id)
	}

	allRecords, err := b.FindAllRecords("items")
	if err != nil {
		b.Logger().Error("Failed to get all records", "error", err)
		return triggerRebuild, err
	}

	for _, record := range allRecords {
		bggId := record.Get("bgg_id").(string)
		if bggId == "" {
			continue
		}

		if !processedItems[bggId] {
			b.Logger().Info(fmt.Sprintf("Marking %s (%s) as deleted", record.Get("name"), bggId))

			record.Set("deleted", true)
			err := b.Save(record)
			if err != nil {
				b.Logger().Error(
					fmt.Sprintf(
						"Failed to mark %s (%s) as deleted",
						record.Get("name"),
						bggId,
					),
					"error",
					err,
				)
			}
		}
	}

	return triggerRebuild, err
}

// FetchBGGWishlistItems fetches the wishlist items for the specified username from Board Game Geek.
func (b *Backend) FetchBGGWishlistItems(username string) ([]BGGItem, error) {
	i := 0
	url := "https://boardgamegeek.com/xmlapi2/collection?username=" + username + "&wishlist=1&stats=1"
	for {
		items, err := b.fetchBGGData(url)
		if err != nil {
			return nil, err
		}

		if items != nil {
			return items, nil
		}

		i++
		if i > 10 {
			break
		}
	}

	return nil, errors.New("failed to fetch data from BGG")
}

func (b *Backend) fetchBGGData(url string) ([]BGGItem, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusAccepted {
		b.Logger().Info("Request accepted, retrying in 3 seconds...")
		time.Sleep(3 * time.Second)
		return nil, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var root BGGRootXML
	err = xml.Unmarshal(body, &root)
	if err != nil {
		return nil, err
	}

	return root.Items, nil
}

// BGGRootXML represents the root element of the XML response from Board Game Geek.
type BGGRootXML struct {
	// Items is a list of BGGItem elements.
	Items []BGGItem `xml:"item" json:"items"`
}

// BGGItem represents an individual item from Board Game Geek.
type BGGItem struct {
	// ObjectType is the type of the object.
	ObjectType string `xml:"objecttype,attr" json:"object_type"`
	// ObjectID is the ID of the object.
	ObjectID string `xml:"objectid,attr" json:"object_id"`
	// SubType is the subtype of the object.
	SubType string `xml:"subtype,attr" json:"sub_type"`
	// CollID is the collection ID.
	CollID string `xml:"collid,attr" json:"coll_id"`
	// Name is the name of the item.
	Name string `xml:"name" json:"name"`
	// YearPublished is the year the item was published.
	YearPublished string `xml:"yearpublished" json:"year_published"`
	// Image is the URL of the item's image.
	Image string `xml:"image" json:"image"`
	// Thumbnail is the URL of the item's thumbnail.
	Thumbnail string `xml:"thumbnail" json:"thumbnail"`
	// Stats contains statistical information about the item.
	Stats BGGStats `xml:"stats" json:"stats"`
	// Status contains the status information of the item.
	Status BGGStatus `xml:"status" json:"status"`
	// NumPlays is the number of times the item has been played.
	NumPlays string `xml:"numplays" json:"num_plays"`
}

// BGGStats represents the stats element from Board Game Geek.
type BGGStats struct {
	// MinPlayers is the minimum number of players.
	MinPlayers string `xml:"minplayers,attr" json:"min_players"`
	// MaxPlayers is the maximum number of players.
	MaxPlayers string `xml:"maxplayers,attr" json:"max_players"`
	// MinPlayTime is the minimum playtime in minutes.
	MinPlayTime string `xml:"minplaytime,attr" json:"min_play_time"`
	// MaxPlayTime is the maximum playtime in minutes.
	MaxPlayTime string `xml:"maxplaytime,attr" json:"max_play_time"`
	// PlayingTime is the total playing time in minutes.
	PlayingTime string `xml:"playingtime,attr" json:"playing_time"`
	// NumOwned is the number of copies owned.
	NumOwned string `xml:"numowned,attr" json:"num_owned"`
	// Rating contains rating information.
	Rating BGGRating `xml:"rating" json:"rating"`
}

// BGGRating represents the rating element from Board Game Geek.
type BGGRating struct {
	// Value is the rating value.
	Value string `xml:"value,attr" json:"value"`
	// UsersRated is the number of users who rated.
	UsersRated BGGRatingValue `xml:"usersrated" json:"users_rated"`
	// Average is the average rating.
	Average BGGRatingValue `xml:"average" json:"average"`
	// BayesAverage is the Bayesian average rating.
	BayesAverage BGGRatingValue `xml:"bayesaverage" json:"bayes_average"`
	// StdDev is the standard deviation of the ratings.
	StdDev BGGRatingValue `xml:"stddev" json:"std_dev"`
	// Median is the median rating.
	Median BGGRatingValue `xml:"median" json:"median"`
}

// BGGRatingValue represents the rating value element from Board Game Geek.
type BGGRatingValue struct {
	// Value is the rating value.
	Value string `xml:"value,attr" json:"value"`
}

// BGGStatus represents the status element from Board Game Geek.
type BGGStatus struct {
	// Own indicates if the item is owned.
	Own string `xml:"own,attr" json:"own"`
	// PrevOwned indicates if the item was previously owned.
	PrevOwned string `xml:"prevowned,attr" json:"prev_owned"`
	// ForTrade indicates if the item is for trade.
	ForTrade string `xml:"fortrade,attr" json:"for_trade"`
	// Want indicates if the item is wanted.
	Want string `xml:"want,attr" json:"want"`
	// WantToPlay indicates if the item is wanted to play.
	WantToPlay string `xml:"wanttoplay,attr" json:"want_to_play"`
	// WantToBuy indicates if the item is wanted to buy.
	WantToBuy string `xml:"wanttobuy,attr" json:"want_to_buy"`
	// Wishlist indicates if the item is on the wishlist.
	Wishlist string `xml:"wishlist,attr" json:"wishlist"`
	// WishlistPriority is the priority of the item on the wishlist.
	WishlistPriority string `xml:"wishlistpriority,attr" json:"wishlist_priority"`
	// PreOrdered indicates if the item is pre-ordered.
	PreOrdered string `xml:"preordered,attr" json:"pre_ordered"`
	// LastModified is the last modified date.
	LastModified string `xml:"lastmodified,attr" json:"last_modified"`
}
