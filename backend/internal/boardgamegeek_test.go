package internal

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"testing"
)

func loadCollectionFixture(t *testing.T) BGGRootXML {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("example", "collection.xml"))
	if err != nil {
		t.Fatalf("read collection.xml: %v", err)
	}
	var root BGGRootXML
	if err := xml.Unmarshal(data, &root); err != nil {
		t.Fatalf("unmarshal collection.xml: %v", err)
	}
	if len(root.Items) == 0 {
		t.Fatal("collection.xml fixture has no items")
	}
	return root
}

func loadThingFixture(t *testing.T) BGGThingRootXML {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("example", "thing.xml"))
	if err != nil {
		t.Fatalf("read thing.xml: %v", err)
	}
	var root BGGThingRootXML
	if err := xml.Unmarshal(data, &root); err != nil {
		t.Fatalf("unmarshal thing.xml: %v", err)
	}
	if len(root.Items) == 0 {
		t.Fatal("thing.xml fixture has no items")
	}
	return root
}

func TestBGGItemToFields_NumericTypes(t *testing.T) {
	root := loadCollectionFixture(t)

	for _, item := range root.Items {
		fields := bggItemToFields(item)

		if _, ok := fields["year_published"].(int); !ok {
			t.Errorf("item %s: year_published is %T, want int", item.ObjectID, fields["year_published"])
		}
		if _, ok := fields["priority"].(int); !ok {
			t.Errorf("item %s: priority is %T, want int", item.ObjectID, fields["priority"])
		}
		if _, ok := fields["playing_time"].(int); !ok {
			t.Errorf("item %s: playing_time is %T, want int", item.ObjectID, fields["playing_time"])
		}
		if _, ok := fields["bgg_rank"].(int); !ok {
			t.Errorf("item %s: bgg_rank is %T, want int", item.ObjectID, fields["bgg_rank"])
		}
		if _, ok := fields["rating"].(float64); !ok {
			t.Errorf("item %s: rating is %T, want float64", item.ObjectID, fields["rating"])
		}
	}
}

func TestBGGItemToFields_KnownItem(t *testing.T) {
	root := loadCollectionFixture(t)

	var sevenWonders *BGGItem
	for i := range root.Items {
		if root.Items[i].ObjectID == "316377" {
			sevenWonders = &root.Items[i]
			break
		}
	}
	if sevenWonders == nil {
		t.Fatal("expected objectid 316377 (7 Wonders Second Edition) in collection.xml fixture")
	}

	fields := bggItemToFields(*sevenWonders)

	if got := fields["year_published"].(int); got != 2020 {
		t.Errorf("year_published = %d, want 2020", got)
	}
	if got := fields["playing_time"].(int); got != 30 {
		t.Errorf("playing_time = %d, want 30", got)
	}
	if got := fields["bgg_rank"].(int); got != 209 {
		t.Errorf("bgg_rank = %d, want 209", got)
	}
	if got := fields["rating"].(float64); got != 7.82698 {
		t.Errorf("rating = %v, want 7.82698", got)
	}
	if got := fields["players"].(string); got != "3-7" {
		t.Errorf("players = %q, want \"3-7\"", got)
	}
	if got := fields["name"].(string); got != "7 Wonders (Second Edition)" {
		t.Errorf("name = %q, want \"7 Wonders (Second Edition)\"", got)
	}
}

func TestBGGThingItemToFields_NumericTypes(t *testing.T) {
	root := loadThingFixture(t)

	for _, item := range root.Items {
		fields := bggThingItemToFields(item)

		if _, ok := fields["minage"].(int); !ok {
			t.Errorf("item %s: minage is %T, want int", item.ID, fields["minage"])
		}
		if _, ok := fields["best_player_count_number"].(int); !ok {
			t.Errorf("item %s: best_player_count_number is %T, want int", item.ID, fields["best_player_count_number"])
		}
		if _, ok := fields["best_player_count"].(string); !ok {
			t.Errorf("item %s: best_player_count is %T, want string", item.ID, fields["best_player_count"])
		}
	}
}

func TestBGGThingItemToFields_KnownItem(t *testing.T) {
	root := loadThingFixture(t)

	var sevenWonders *BGGThingItem
	for i := range root.Items {
		if root.Items[i].ID == "316377" {
			sevenWonders = &root.Items[i]
			break
		}
	}
	if sevenWonders == nil {
		t.Fatal("expected id 316377 (7 Wonders Second Edition) in thing.xml fixture")
	}

	fields := bggThingItemToFields(*sevenWonders)

	if got := fields["minage"].(int); got != 10 {
		t.Errorf("minage = %d, want 10", got)
	}
}
