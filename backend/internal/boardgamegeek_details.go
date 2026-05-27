package internal

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

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

	resp, err := httpClient.Do(req)
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

		resp2, err := httpClient.Do(req)
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
