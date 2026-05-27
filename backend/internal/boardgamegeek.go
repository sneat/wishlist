package internal

import (
	"bytes"
	"database/sql"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/microcosm-cc/bluemonday"
	"github.com/pocketbase/pocketbase/core"
	"golang.org/x/net/html"
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

// FetchBGGWishlistItems fetches the wishlist items for the configured username from Board Game Geek.
func (b *Backend) FetchBGGWishlistItems() ([]BGGItem, error) {
	i := 0
	attempts := 10
	for {
		items, err := b.fetchBGGData()
		if err != nil {
			return nil, err
		}

		if items != nil {
			return items, nil
		}

		i++
		if i > attempts {
			break
		}
	}

	return nil, errors.New(fmt.Sprintf("failed to fetch data from BGG after %d attempts", attempts))
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

	resp, err := http.DefaultClient.Do(req)
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

// BGGThingRootXML represents the root element of the XML response from the BGG Thing API.
type BGGThingRootXML struct {
	// Items is a list of BGGThingItem elements.
	Items []BGGThingItem `xml:"item" json:"items"`
}

// BGGThingItem represents an individual item from the BGG Thing API.
type BGGThingItem struct {
	// ID is the ID of the item.
	ID string `xml:"id,attr" json:"id"`
	// Description is the full description of the item.
	Description string `xml:"description" json:"description"`
	// MinAge is the minimum recommended age. BGG returns it as <minage value="N"/>.
	MinAge BGGRatingValue `xml:"minage" json:"min_age"`
	// Polls contains detailed poll information.
	Polls []BGGPoll `xml:"poll" json:"polls"`
	// PollSummary contains poll summary information.
	PollSummary []BGGPollSummary `xml:"poll-summary" json:"poll_summary"`
	// Links contains category and mechanic links.
	Links []BGGLink `xml:"link" json:"links"`
}

// BGGPoll represents a detailed poll element from BGG.
type BGGPoll struct {
	// Name is the name of the poll.
	Name string `xml:"name,attr" json:"name"`
	// Title is the title of the poll.
	Title string `xml:"title,attr" json:"title"`
	// Results contains poll results grouped by player count.
	Results []BGGPollResults `xml:"results" json:"results"`
}

// BGGPollResults represents results for a specific player count.
type BGGPollResults struct {
	// NumPlayers is the player count (e.g., "3", "4", "5+").
	NumPlayers string `xml:"numplayers,attr" json:"numplayers"`
	// Results contains the vote results (Best, Recommended, Not Recommended).
	Results []BGGPollResultValue `xml:"result" json:"results"`
}

// BGGPollResultValue represents a vote result with its count.
type BGGPollResultValue struct {
	// Value is the result type (e.g., "Best", "Recommended", "Not Recommended").
	Value string `xml:"value,attr" json:"value"`
	// NumVotes is the number of votes for this result.
	NumVotes string `xml:"numvotes,attr" json:"numvotes"`
}

// BGGPollSummary represents a poll summary element from BGG.
type BGGPollSummary struct {
	// Name is the name of the poll.
	Name string `xml:"name,attr" json:"name"`
	// Title is the title of the poll.
	Title string `xml:"title,attr" json:"title"`
	// Results contains poll results.
	Results []BGGPollResult `xml:"result" json:"results"`
}

// BGGPollResult represents a poll result element.
type BGGPollResult struct {
	// Name is the name of the result.
	Name string `xml:"name,attr" json:"name"`
	// Value is the value of the result.
	Value string `xml:"value,attr" json:"value"`
}

// BGGLink represents a link element (categories, mechanics, etc.).
type BGGLink struct {
	// Type is the type of link.
	Type string `xml:"type,attr" json:"type"`
	// Value is the value of the link.
	Value string `xml:"value,attr" json:"value"`
}

// fetchBGGThingDetails fetches detailed information for the given BGG IDs.
// Maximum 20 IDs can be fetched in a single request.
func (b *Backend) fetchBGGThingDetails(bggIDs []string) ([]BGGThingItem, error) {
	if len(bggIDs) == 0 {
		return nil, nil
	}

	if len(bggIDs) > 20 {
		return nil, errors.New("maximum 20 IDs can be fetched at once")
	}

	// Build URL with comma-separated IDs
	ids := strings.Join(bggIDs, ",")
	url := fmt.Sprintf("https://boardgamegeek.com/xmlapi2/thing?id=%s&stats=1", ids)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	// Add auth if available
	if b.bggAuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+b.bggAuthToken)
	} else if b.username != "" && b.password != "" {
		req.AddCookie(&http.Cookie{Name: "bggusername", Value: b.username})
		req.AddCookie(&http.Cookie{Name: "bggpassword", Value: b.password})
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Handle 202 Accepted - BGG is queuing the request
	if resp.StatusCode == http.StatusAccepted {
		b.Logger().Info("BGG returned 202 Accepted, data is being queued", "ids", ids)
		resp.Body.Close() // Close the first response

		// Wait a bit and retry once
		time.Sleep(2 * time.Second)

		resp2, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp2.Body.Close()

		if resp2.StatusCode == http.StatusAccepted {
			// Still 202 after retry, skip this batch for now
			b.Logger().Warn("BGG still returning 202 after retry, skipping batch", "ids", ids)
			return nil, fmt.Errorf("BGG returned 202 Accepted after retry")
		}

		resp = resp2
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var root BGGThingRootXML
	err = xml.Unmarshal(body, &root)
	if err != nil {
		return nil, err
	}

	return root.Items, nil
}

// sanitizeHTML sanitizes HTML content or converts plain text newlines to HTML.
func sanitizeHTML(html string) string {
	// Check if the description contains HTML tags
	hasHTML := strings.Contains(html, "<") && strings.Contains(html, ">")

	if hasHTML {
		// Create custom policy for HTML content
		p := bluemonday.NewPolicy()

		// Allow safe formatting tags
		p.AllowElements("b", "i", "em", "strong", "br", "p", "ul", "ol", "li")

		// Sanitize and return
		return p.Sanitize(html)
	}

	// Plain text: convert newlines to <br> tags
	// First, trim leading/trailing whitespace
	text := strings.TrimSpace(html)

	// Replace multiple consecutive newlines with paragraph breaks
	text = strings.ReplaceAll(text, "\n\n", "</p><p>")

	// Replace single newlines with <br>
	text = strings.ReplaceAll(text, "\n", "<br>")

	// Wrap in paragraph tags
	text = "<p>" + text + "</p>"

	return text
}

// truncateHTML truncates HTML content to fit within maxLen characters while ensuring valid HTML structure.
// It appends "..." to indicate truncation. The function attempts to truncate intelligently, accounting for
// closing tags that need to be added to maintain valid HTML.
func truncateHTML(htmlContent string, maxLen int) string {
	// If already within limit, return as-is
	if len(htmlContent) <= maxLen {
		return htmlContent
	}

	ellipsis := "..."
	closingPTag := "</p>"
	commonSuffix := ellipsis + closingPTag
	targetLen := maxLen - len(commonSuffix)

	// Most content come as raw strings that we convert to basic HTML.
	truncated := htmlContent[:targetLen]
	validHTML := parseAndCloseHTMLTags(truncated + ellipsis)

	if len(validHTML) <= maxLen {
		return validHTML
	}

	// If the closing tags pushed us over, reduce by that amount.
	excess := len(validHTML) - targetLen
	if targetLen-excess > 0 {
		truncated = htmlContent[:targetLen-excess]
		validHTML = parseAndCloseHTMLTags(truncated + ellipsis)

		if len(validHTML) <= maxLen {
			return validHTML
		}
	}

	// Fallback to trimming significantly shorter than the maximum.
	safeLen := int(0.9 * float64(maxLen))
	if safeLen > len(htmlContent) {
		safeLen = len(htmlContent)
	}
	truncated = htmlContent[:safeLen]
	validHTML = parseAndCloseHTMLTags(truncated + ellipsis)

	return validHTML
}

// parseAndCloseHTMLTags parses an HTML fragment and returns valid HTML with all tags properly closed.
// The html.Parse function automatically closes any unclosed tags during parsing and rendering.
func parseAndCloseHTMLTags(htmlFragment string) string {
	doc, err := html.Parse(strings.NewReader(htmlFragment))
	if err != nil {
		// Can't parse - return as-is (shouldn't happen with sanitized input)
		return htmlFragment
	}

	// Find body element (parser wraps fragments in <html><body>)
	body := findBodyNode(doc)
	if body == nil {
		return htmlFragment
	}

	// Render just the body's children (excludes <html><body> wrapper)
	var buf bytes.Buffer
	for child := body.FirstChild; child != nil; child = child.NextSibling {
		html.Render(&buf, child)
	}

	return buf.String()
}

// findBodyNode recursively searches for the body element in an HTML node tree.
func findBodyNode(n *html.Node) *html.Node {
	if n.Type == html.ElementNode && n.Data == "body" {
		return n
	}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if result := findBodyNode(child); result != nil {
			return result
		}
	}
	return nil
}

// extractLinksByType extracts link values by link type (e.g., "boardgamecategory", "boardgamemechanic").
func extractLinksByType(links []BGGLink, linkType string) []string {
	var result []string
	for _, link := range links {
		if link.Type == linkType {
			result = append(result, link.Value)
		}
	}
	return result
}

// extractNumericPlayerCount extracts the numeric portion from player count strings.
// Examples: "7" -> "7", "7+" -> "7", "3-4" -> "3", "10+" -> "10"
func extractNumericPlayerCount(playerCount string) string {
	// Extract all digits from the string
	var numStr strings.Builder
	for _, ch := range playerCount {
		if ch >= '0' && ch <= '9' {
			numStr.WriteRune(ch)
		} else if numStr.Len() > 0 {
			// Stop at first non-digit after we've found digits
			break
		}
	}
	return numStr.String()
}

// extractPlayerRecommendations extracts player count recommendations from polls and poll summaries.
// Returns: bestPlayerCountDisplay (text from poll-summary), bestPlayerCountNumber (single number with most Best votes).
func extractPlayerRecommendations(polls []BGGPoll, pollSummaries []BGGPollSummary) (string, string) {
	// Get display text from poll-summary
	bestDisplay := ""
	recommendedDisplay := ""
	for _, summary := range pollSummaries {
		if summary.Name == "suggested_numplayers" {
			for _, result := range summary.Results {
				if strings.Contains(strings.ToLower(result.Value), "no votes") || strings.Contains(strings.ToLower(result.Value), "undetermined") {
					continue // Skip unwanted values
				}
				if result.Name == "bestwith" {
					bestDisplay = result.Value
				} else if result.Name == "recommmendedwith" || result.Name == "recommendedwith" { // Note: BGG uses triple 'm'
					recommendedDisplay = result.Value
				}
			}
		}
	}

	// Track the player count with the highest "Best" votes
	type playerCountVotes struct {
		playerCount string
		bestVotes   int
	}
	var bestCandidate playerCountVotes
	var recommendedCandidate playerCountVotes

	for _, poll := range polls {
		if poll.Name == "suggested_numplayers" {
			for _, results := range poll.Results {
				playerCountNum := extractNumericPlayerCount(results.NumPlayers)
				if playerCountNum == "" {
					continue
				}

				bestVotes := 0
				recommendedVotes := 0
				notRecommendedVotes := 0

				for _, result := range results.Results {
					votes, _ := strconv.Atoi(result.NumVotes)
					if result.Value == "Best" {
						bestVotes = votes
					} else if result.Value == "Recommended" {
						recommendedVotes = votes
					} else if result.Value == "Not Recommended" {
						notRecommendedVotes = votes
					}
				}

				// Check if this player count qualifies as "Best"
				// A player count is "Best" if:
				// - Best votes > Recommended votes
				// - Best votes > Not Recommended votes
				// - Best votes > 0
				if bestVotes > recommendedVotes && bestVotes > notRecommendedVotes && bestVotes > 0 {
					if bestVotes > bestCandidate.bestVotes {
						bestCandidate = playerCountVotes{
							playerCount: playerCountNum,
							bestVotes:   bestVotes,
						}
					}
				} else if recommendedVotes > bestVotes && recommendedVotes > notRecommendedVotes && recommendedVotes > 0 {
					if recommendedVotes > recommendedCandidate.bestVotes {
						recommendedCandidate = playerCountVotes{
							playerCount: playerCountNum,
							bestVotes:   recommendedVotes,
						}
					}
				}
			}
		}
	}

	// Prefer best display text, fall back to recommended
	displayText := bestDisplay
	if displayText == "" {
		displayText = recommendedDisplay
	}

	// Prefer best number, fall back to recommended
	number := bestCandidate.playerCount
	if number == "" {
		number = recommendedCandidate.playerCount
	}

	return displayText, number
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

// bggThingItemToFields converts a BGGThingItem (from the thing XML) into the field-name → value
// map that fetchThingDetailsForItems writes onto an items record. The best_player_count_number
// key uses the singular name introduced by the numeric-fields migration.
func bggThingItemToFields(item BGGThingItem) map[string]any {
	description := ""
	if item.Description != "" {
		description = truncateHTML(sanitizeHTML(item.Description), 5000)
	}

	bestPlayerCount, bestPlayerCountNumber := extractPlayerRecommendations(item.Polls, item.PollSummary)

	return map[string]any{
		"description":              description,
		"minage":                   parseIntOrZero(item.MinAge.Value),
		"best_player_count":        bestPlayerCount,
		"best_player_count_number": parseIntOrZero(bestPlayerCountNumber),
		"categories":               extractLinksByType(item.Links, "boardgamecategory"),
		"mechanics":                extractLinksByType(item.Links, "boardgamemechanic"),
	}
}

// fetchThingDetailsForItems fetches detailed information for items that need it.
// If forceRefresh is true, all items are fetched regardless of staleness.
// If limit > 0, only that many items are fetched.
// Returns true if the frontend should be rebuilt.
func (b *Backend) fetchThingDetailsForItems(forceRefresh bool, limit int) (bool, error) {
	var triggerRebuild bool

	start := time.Now()
	b.Logger().Info("Starting thing details fetch", "force_refresh", forceRefresh, "limit", limit)

	// Build filter for items that need details
	var filter string
	if forceRefresh {
		filter = "" // All items
	} else {
		cutoffDate := time.Now().AddDate(0, 0, -30)
		filter = fmt.Sprintf("details_last_fetched = null || details_last_fetched < '%s'", cutoffDate.Format("2006-01-02 15:04:05"))
	}

	// Query items using PocketBase API
	records, err := b.FindRecordsByFilter(
		"items",
		filter,
		"id",
		limit,
		0,
	)
	if err != nil {
		return triggerRebuild, fmt.Errorf("failed to query items: %w", err)
	}

	if len(records) == 0 {
		b.Logger().Info("No items need details fetching")
		return triggerRebuild, nil
	}

	b.Logger().Info("Found items needing details", "count", len(records))

	// Process in batches of 20
	totalProcessed := 0
	for i := 0; i < len(records); i += 20 {
		// Add delay between batches (except for first batch)
		if i > 0 {
			b.Logger().Debug("Waiting 5 seconds before next batch")
			time.Sleep(5 * time.Second)
		}

		// Get batch of up to 20 items
		end := i + 20
		if end > len(records) {
			end = len(records)
		}
		batch := records[i:end]

		bggIDs := make([]string, len(batch))
		for j, record := range batch {
			bggIDs[j] = record.GetString("bgg_id")
		}

		b.Logger().Info("Fetching thing details batch", "batch", i/20+1, "ids", bggIDs)

		thingItems, err := b.fetchBGGThingDetails(bggIDs)
		if err != nil {
			b.Logger().Error("Failed to fetch thing details", "error", err, "batch", i/20+1)
			continue
		}

		// Update records
		for _, thingItem := range thingItems {
			record, err := b.FindFirstRecordByData("items", "bgg_id", thingItem.ID)
			if err != nil {
				b.Logger().Warn("Failed to find record for BGG ID", "bgg_id", thingItem.ID, "error", err)
				continue
			}

			for k, v := range bggThingItemToFields(thingItem) {
				record.Set(k, v)
			}
			record.Set("details_last_fetched", time.Now())

			if err = b.Save(record); err != nil {
				b.Logger().Error("Failed to save record", "bgg_id", thingItem.ID, "error", err)
				continue
			}

			totalProcessed++
			triggerRebuild = true
		}
	}

	b.Logger().
		With("duration", time.Since(start).String()).
		With("processed", totalProcessed).
		With("total", len(records)).
		Info("Thing details fetch completed")

	return triggerRebuild, nil
}
