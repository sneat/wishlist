package internal

import (
	"encoding/json"
	"testing"
	"time"
)

// A minimal BGO batch response: input "0" is the pricehistory.list array,
// input "1" is the pricestats.get summary object. One history point has min 0
// and must be dropped by compaction.
const bgoPricingFixture = `[
  {"result":{"data":[
    {"id":"a","dt":"2026-03-01T00:00:00.000Z","max":110,"mean":95.0,"min":77.5,"min_st":"Gamerholic"},
    {"id":"b","dt":"2026-03-02T00:00:00.000Z","max":116.4,"mean":95.8,"min":80.2,"min_st":"Gamerholic"},
    {"id":"c","dt":"2026-03-03T00:00:00.000Z","max":0,"mean":0,"min":0,"min_st":""},
    {"id":"d","dt":"2026-03-04T00:00:00.000Z","max":120,"mean":99.0,"min":82.9,"min_st":"Store2"}
  ]}},
  {"result":{"data":{"id":"sum1","latest":{"min":82.9,"max":120,"mean":99.0,"median":99.0}}}}
]`

func mustTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestBGOPriceResult_ParsesHistoryAndSummary(t *testing.T) {
	var responses []BGOPriceResponse
	if err := json.Unmarshal([]byte(bgoPricingFixture), &responses); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	if len(responses) != 2 {
		t.Fatalf("expected 2 responses, got %d", len(responses))
	}
	if got := len(responses[0].Result.History); got != 4 {
		t.Errorf("history length = %d, want 4 (raw points, before compaction)", got)
	}
	if responses[1].Result.PriceSummary.ID != "sum1" {
		t.Errorf("summary ID = %q, want \"sum1\"", responses[1].Result.PriceSummary.ID)
	}
}

func TestCompactPriceHistory_DropsZerosAndRounds(t *testing.T) {
	points := []BGOPricePoint{
		{Date: mustTime("2026-03-01T00:00:00.000Z"), MinPrice: 77.5},
		{Date: mustTime("2026-03-02T00:00:00.000Z"), MinPrice: 80.2},
		{Date: mustTime("2026-03-03T00:00:00.000Z"), MinPrice: 0},
		{Date: mustTime("2026-03-04T00:00:00.000Z"), MinPrice: 82.9},
	}

	got := compactPriceHistory(points)

	if len(got) != 3 {
		t.Fatalf("compacted length = %d, want 3 (zero dropped)", len(got))
	}
	if got[0].Date != "2026-03-01" || got[0].Price != 78 {
		t.Errorf("point 0 = %+v, want {Date:2026-03-01, Price:78}", got[0])
	}
	if got[2].Date != "2026-03-04" || got[2].Price != 83 {
		t.Errorf("point 2 = %+v, want {Date:2026-03-04, Price:83}", got[2])
	}
}
