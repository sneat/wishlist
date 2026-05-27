package internal

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/pocketbase/pocketbase/core"
)

func (b *Backend) autoPopulateBGOIds() error {
	records, err := b.FindRecordsByFilter(
		"items",
		"bgo_id = ''",
		"",
		0,
		0,
	)
	if err != nil {
		return err
	}

	for _, record := range records {
		if err = b.autoPopulateBGOId(record); err != nil {
			b.Logger().Error("Failed to auto-populate Board Game Oracle ID", "error", err)
		}
	}

	return nil
}

func (b *Backend) autoPopulateBGOId(bggRecord *core.Record) error {
	name := bggRecord.GetString("name")
	bgoId := bggRecord.GetString("bgo_id")
	if bgoId != "" {
		b.Logger().
			With("bgo_id", bgoId).
			With("name", name).
			Info("Board Game Oracle ID already set, skipping auto-population")
		return nil
	}

	if name == "" {
		return errors.New("name is empty")
	}

	u, err := url.Parse("https://www.boardgameoracle.com/api/trpc/boardgame.suggestion")
	if err != nil {
		return errors.New("failed to generate BGO population URL")
	}

	// Board Game Oracle seems to only like receiving the first 3 words.
	nameToLookup := sanitiseName(name)
	words := strings.Fields(nameToLookup)
	if len(words) > 3 {
		words = words[:3]
	}
	nameToLookup = strings.Join(words, " ")

	type suggestionRequestData struct {
		Query string `json:"q"`
		Limit int    `json:"limit"`
	}
	requestData := make(map[string]suggestionRequestData)
	requestData["0"] = suggestionRequestData{
		Query: nameToLookup,
		Limit: 25,
	}

	encodedData, err := json.Marshal(requestData)
	if err != nil {
		return fmt.Errorf("failed to encode suggestion request data: %w", err)
	}

	q := u.Query()
	q.Set("batch", "1")
	q.Set("input", string(encodedData))
	u.RawQuery = q.Encode()

	resp, err := httpClient.Get(u.String())
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var response []BGOSuggestionResponse
	if err = json.Unmarshal(body, &response); err != nil {
		return err
	}

	if len(response) == 0 {
		b.Logger().With("name", name).Error("No suggestions found")
		return errors.New("no suggestions found")
	}

	selectedId := b.pickSuggestion(name, response[0].Result.Data.Items)
	if selectedId == "" {
		b.Logger().With("name", name).Error("No accurate suggestions found")
		return errors.New("no suggestion found")
	}

	bggRecord.Set("bgo_id", selectedId)
	if err = b.Save(bggRecord); err != nil {
		b.Logger().Error("Failed to save BGO ID", "error", err)
		return err
	}

	return nil
}

func (b *Backend) pickSuggestion(requestedName string, suggestions []BGOSuggestion) string {
	if len(suggestions) == 1 {
		return suggestions[0].Key
	}

	// Sort the suggestions by score in descending order.
	sort.Slice(suggestions, func(i, j int) bool {
		return suggestions[i].Score > suggestions[j].Score
	})

	sanitisedName := sanitiseName(requestedName)

	for _, suggestion := range suggestions {
		if sanitiseName(suggestion.Title) == sanitisedName {
			return suggestion.Key
		}
	}

	return ""
}

// sanitiseName sanitises the name by removing all non-alphanumeric characters and converting to lowercase.
func sanitiseName(name string) string {
	var builder strings.Builder
	for _, r := range name {
		if unicode.IsLetter(r) ||
			unicode.IsNumber(r) ||
			unicode.IsSpace(r) ||
			r == '\'' ||
			r == '.' {
			builder.WriteRune(unicode.ToLower(r))
		}
	}
	return strings.TrimSpace(builder.String())
}

func (b *Backend) syncBGOPrices() bool {
	var triggerRebuild bool
	if b.countryCode == "" {
		b.Logger().Error("Country code not set, skipping BGO price sync")
		return triggerRebuild
	}

	start := time.Now()
	b.Logger().Info("Syncing BGO prices.")

	prices, err := b.fetchBGOPricingData()
	if err != nil {
		b.Logger().Error("Failed to sync BGO prices", "error", err)
		return triggerRebuild
	}

	triggerRebuild, err = b.processBGOPrices(prices)
	if err != nil {
		b.Logger().Error("Failed to process BGO prices", "error", err)
		return triggerRebuild
	}

	b.Logger().
		With("duration", time.Since(start).String()).
		With("count", len(prices)).
		Info("Syncing BGO prices completed.")

	return triggerRebuild
}

// processBGOPrices processes the BGO prices and updates the records.
// It returns a boolean indicating whether a frontend rebuild is required.
func (b *Backend) processBGOPrices(prices []*BGOPriceSummary) (bool, error) {
	var triggerRebuild bool
	for _, price := range prices {
		b.Logger().Debug(fmt.Sprintf("Processing BGO price for %s", price.BgoId))

		err := b.RunInTransaction(func(txApp core.App) error {
			record, err := b.FindFirstRecordByData("items", "bgo_id", price.BgoId)
			if err != nil || record == nil {
				b.Logger().Warn(fmt.Sprintf("Failed to find record %s", price.BgoId), "error", err)
				return nil
			}

			roundedValue := int(math.Round(price.Latest.Min))
			if roundedValue == 0 {
				return nil
			}

			oldPrice := record.GetInt("price")
			if oldPrice != roundedValue {
				b.Logger().
					With("old_price", oldPrice).
					With("new_price", roundedValue).
					With("name", record.GetString("name")).
					Info("Price changed")
				triggerRebuild = true
			}

			record.Set("price", roundedValue)

			if err = txApp.Save(record); err != nil {
				return err
			}

			return nil
		})

		if err != nil {
			b.Logger().Error(fmt.Sprintf("Failed to process %s", price.BgoId), "error", err)
		}
	}

	return triggerRebuild, nil
}

func (b *Backend) fetchBGOPricingData() ([]*BGOPriceSummary, error) {
	response := make([]*BGOPriceSummary, 0)

	records, err := b.FindRecordsByFilter(
		"items",
		"bgo_id != '' && deleted = false",
		"",
		0,
		0,
	)
	if err != nil {
		return nil, err
	}

	for _, record := range records {
		if tmpRsp, err := b.fetchBGOPricingDataForID(record.GetString("bgo_id")); err != nil {
			b.Logger().
				With("name", record.GetString("name")).
				Error("Failed to fetch BGO pricing data", "error", err)
		} else {
			response = append(response, tmpRsp)
		}
	}

	return response, nil
}

func (b *Backend) fetchBGOPricingDataForID(bgoId string) (*BGOPriceSummary, error) {
	u, err := url.Parse("https://www.boardgameoracle.com/api/trpc/pricehistory.list,pricestats.get")
	if err != nil {
		return nil, errors.New("failed to generate BGO pricing URL")
	}

	// BgoPriceRequestData represents the Board Game Oracle price request data.
	type BgoPriceRequestData struct {
		// Region is the region code.
		Region string `json:"region"`
		// Key is the game id.
		Key string `json:"key"`
		// Range is the length of time to fetch data for. '7d' | '1m' | '3m' | '1y' | 'max'
		Range string `json:"range,omitempty"`
	}

	requestData := make(map[string]BgoPriceRequestData)
	requestData["0"] = BgoPriceRequestData{
		Region: b.countryCode,
		Key:    bgoId,
		Range:  "7d",
	}
	requestData["1"] = BgoPriceRequestData{
		Region: b.countryCode,
		Key:    bgoId,
	}

	encodedData, err := json.Marshal(requestData)
	if err != nil {
		return nil, fmt.Errorf("failed to encode pricing request data: %w", err)
	}

	q := u.Query()
	q.Set("batch", "1")
	q.Set("input", string(encodedData))
	u.RawQuery = q.Encode()

	resp, err := httpClient.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var responses []BGOPriceResponse
	if err = json.Unmarshal(body, &responses); err != nil {
		return nil, err
	}

	for _, response := range responses {
		if response.Result.PriceSummary.ID != "" {
			response.Result.PriceSummary.BgoId = bgoId

			return &response.Result.PriceSummary, nil
		}
	}

	b.Logger().With("response", responses).Error("No price summary found in response")

	return nil, errors.New("no price summary found in response")
}

// BGOSuggestion represents an individual item being suggested.
type BGOSuggestion struct {
	// Type is the type of the item.
	Type string `json:"type"`
	// YearPublished is the year the item was published.
	YearPublished *int `json:"year_published"`
	// Title is the title of the item.
	Title string `json:"title"`
	// Slug is the slug of the item.
	Slug string `json:"slug"`
	// Key is the primary identifier of the item.
	Key string `json:"key"`
	// Score is the score of the item.
	Score float64 `json:"score"`
}

// BGOSuggestionResult represents the result structure.
type BGOSuggestionResult struct {
	// Data contains the data.
	Data struct {
		// Items is a list of items.
		Items []BGOSuggestion `json:"items"`
		// TotalResults is the total number of results.
		TotalResults int `json:"totalResults"`
	} `json:"data"`
}

// BGOSuggestionResponse represents the top-level response structure of the suggestion request.
type BGOSuggestionResponse struct {
	// Result contains the result data.
	Result BGOSuggestionResult `json:"result"`
}

// BGOPriceResponse represents the top-level response structure of the pricing request.
type BGOPriceResponse struct {
	// Result contains the result data.
	Result BGOPriceResult `json:"result"`
}

// BGOPriceResult represents the result structure.
type BGOPriceResult struct {
	// Data contains the data, which can be either a slice of price points (which are ignored) or a BGOPriceSummary.
	Data interface{} `json:"data"`
	// PriceSummary contains the price summary.
	PriceSummary BGOPriceSummary `json:"price_summary"`
}

func (d *BGOPriceResult) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	// Unmarshal the data field only if it's not an array.
	if val, ok := raw["data"]; ok && len(val) > 0 && val[0] != '[' {
		var summary BGOPriceSummary
		if err := json.Unmarshal(val, &summary); err != nil {
			return err
		}

		d.PriceSummary = summary
	}

	return nil
}

// BGOLatestPrices represents the latest data summary.
type BGOLatestPrices struct {
	// Max is the maximum value.
	Max float64 `json:"max"`
	// Mean is the mean value.
	Mean float64 `json:"mean"`
	// Median is the median value.
	Median float64 `json:"median"`
	// Min is the minimum value.
	Min float64 `json:"min"`
}

// BGOPriceDrop represents the price drop change.
type BGOPriceDrop struct {
	// ChangePercent is the percentage change in price.
	ChangePercent float64 `json:"change_percent"`
}

// BGOPriceSummary represents a summary of the board game prices.
type BGOPriceSummary struct {
	// BgoId is the Board Game Oracle ID. It gets set based on the request as it is not part of the response.
	BgoId string `json:"bgo_id"`
	// ID is the unique identifier for the price summary.
	ID string `json:"id"`
	// Latest contains the latest price summary.
	Latest BGOLatestPrices `json:"latest"`
	// PriceDropDay contains the price drop data for the day.
	PriceDropDay BGOPriceDrop `json:"price_drop_day"`
	// PriceDropWeek contains the price drop data for the week.
	PriceDropWeek BGOPriceDrop `json:"price_drop_week"`
	// Lowest30d is the lowest value in the last 30 days.
	Lowest30d float64 `json:"lowest_30d"`
	// Lowest52w is the lowest value in the last 52 weeks.
	Lowest52w float64 `json:"lowest_52w"`
	// Lowest30dDate is the date of the lowest value in the last 30 days.
	Lowest30dDate time.Time `json:"lowest_30d_date"`
	// Lowest30dStore is the store with the lowest value in the last 30 days.
	Lowest30dStore string `json:"lowest_30d_store"`
	// Lowest52wDate is the date of the lowest value in the last 52 weeks.
	Lowest52wDate time.Time `json:"lowest_52w_date"`
	// Lowest52wStore is the store with the lowest value in the last 52 weeks.
	Lowest52wStore string `json:"lowest_52w_store"`
}
