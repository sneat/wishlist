package internal

import (
	"database/sql"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

func (b *Backend) syncBGGWishlist() bool {
	var triggerRebuild bool
	if b.username == "" {
		b.Logger().Error("No BGG username provided.")
		return triggerRebuild
	}

	start := time.Now()
	b.Logger().Info("Syncing BGG wishlist items.")

	items, err := b.FetchBGGWishlistItems()
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

	newItems := make([]*core.Record, 0)

	for _, item := range items {
		b.Logger().Debug(fmt.Sprintf("Processing %s (%s)", item.Name, item.ObjectID))

		if err = b.RunInTransaction(func(txApp core.App) error {
			record, err := b.FindFirstRecordByData("items", "bgg_id", item.ObjectID)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				b.Logger().Warn(fmt.Sprintf("Failed to find record %s (%s)", item.Name, item.ObjectID), "error", err)
			}

			isNewRecord := false
			if record == nil {
				isNewRecord = true
				record = core.NewRecord(collection)
				newItems = append(newItems, record)
			}

			for k, v := range bggItemToFields(item) {
				record.Set(k, v)
			}
			isOwned := item.Status.Own == "1"
			if record.GetBool("is_owned") != isOwned {
				newItems = append(newItems, record)
			}
			record.Set("is_owned", isOwned)
			record.Set("last_modified", item.Status.GetLastModified().Format("2006-01-02 15:04:05.000Z"))
			if isNewRecord {
				record.Set("created_at", item.Status.GetLastModified().Format("2006-01-02 15:04:05.000Z"))
			}

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

	return triggerRebuild, err
}

// errBGGAccepted signals that BGG returned HTTP 202 (the collection request is being
// prepared) and the call should be retried after a backoff.
var errBGGAccepted = errors.New("bgg request accepted, retry")

// httpClient is the shared client for all outbound BGG/BGO requests. The timeout bounds
// the whole request (connect + body read) so a hung upstream can't stall a sync forever.
var httpClient = &http.Client{Timeout: 30 * time.Second}

const (
	bggRetryBaseDelay = 2 * time.Second
	bggRetryMaxDelay  = 30 * time.Second
	bggMaxAttempts    = 8
)

// bggRetryBackoff returns the delay before the given (zero-based) retry attempt:
// 2s doubling per attempt, capped at 30s. Overflow at large attempts is guarded to the cap.
func bggRetryBackoff(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	d := bggRetryBaseDelay << attempt
	if d <= 0 || d > bggRetryMaxDelay {
		return bggRetryMaxDelay
	}
	return d
}

// FetchBGGWishlistItems fetches the wishlist items for the configured username from Board Game Geek.
func (b *Backend) FetchBGGWishlistItems() ([]BGGItem, error) {
	for attempt := range bggMaxAttempts {
		items, err := b.fetchBGGData()
		if errors.Is(err, errBGGAccepted) {
			delay := bggRetryBackoff(attempt)
			b.Logger().
				With("attempt", attempt+1).
				With("delay", delay.String()).
				Info("BGG request accepted, retrying")
			time.Sleep(delay)
			continue
		}
		if err != nil {
			return nil, err
		}
		return items, nil
	}

	return nil, fmt.Errorf("failed to fetch data from BGG after %d attempts", bggMaxAttempts)
}

func (b *Backend) fetchBGGData() ([]BGGItem, error) {
	url := fmt.Sprintf("https://boardgamegeek.com/xmlapi2/collection?username=%s&stats=1", b.username)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	if b.bggAuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+b.bggAuthToken)
	} else if b.username != "" && b.password != "" {
		req.AddCookie(&http.Cookie{Name: "bggusername", Value: b.username})
		req.AddCookie(&http.Cookie{Name: "bggpassword", Value: b.password})
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusAccepted {
		return nil, errBGGAccepted
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
	// Ranks contains ranking information.
	Ranks BGGRanks `xml:"ranks" json:"ranks"`
}

// BGGRanks represents the ranks element from Board Game Geek.
type BGGRanks struct {
	// Rank is a list of rank elements.
	Rank []BGGRank `xml:"rank" json:"rank"`
}

// BGGRank represents an individual rank element from Board Game Geek.
type BGGRank struct {
	// Type is the type of rank.
	Type string `xml:"type,attr" json:"type"`
	// ID is the ID of the rank.
	ID string `xml:"id,attr" json:"id"`
	// Name is the name of the rank.
	Name string `xml:"name,attr" json:"name"`
	// FriendlyName is the friendly name of the rank.
	FriendlyName string `xml:"friendlyname,attr" json:"friendly_name"`
	// Value is the rank value.
	Value string `xml:"value,attr" json:"value"`
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

// GetLastModified parses the LastModified field and returns it as a time.Time.
func (s *BGGStatus) GetLastModified() time.Time {
	if s.LastModified == "" {
		return time.Time{}
	}

	t, err := time.ParseInLocation("2006-01-02 15:04:05", s.LastModified, time.UTC)
	if err != nil {
		return time.Time{}
	}

	return t
}

// bggItemToFields converts a BGGItem (from the collection XML) into the field-name → value
// map that processBGGItems writes onto an items record. Numeric values are returned as
// int / float64 so they round-trip cleanly through NumberField columns.
func bggItemToFields(item BGGItem) map[string]any {
	minPlayers := parseIntOrZero(item.Stats.MinPlayers)
	maxPlayers := parseIntOrZero(item.Stats.MaxPlayers)

	players := ""
	switch {
	case minPlayers != 0 && maxPlayers != 0:
		players = fmt.Sprintf("%d-%d", minPlayers, maxPlayers)
	case minPlayers != 0:
		players = fmt.Sprintf("%d+", minPlayers)
	case maxPlayers != 0:
		players = fmt.Sprintf("Up to %d", maxPlayers)
	}

	bggRank := 0
	for _, rank := range item.Stats.Rating.Ranks.Rank {
		if rank.Name == "boardgame" && rank.Value != "Not Ranked" {
			bggRank = parseIntOrZero(rank.Value)
			break
		}
	}

	return map[string]any{
		"name":           item.Name,
		"thumbnail":      item.Thumbnail,
		"image":          item.Image,
		"year_published": parseIntOrZero(item.YearPublished),
		"players":        players,
		"rating":         parseFloatOrZero(item.Stats.Rating.Average.Value),
		"priority":       parseIntOrZero(item.Status.WishlistPriority),
		"playing_time":   parseIntOrZero(item.Stats.PlayingTime),
		"bgg_id":         item.ObjectID,
		"bgg_rank":       bggRank,
	}
}
