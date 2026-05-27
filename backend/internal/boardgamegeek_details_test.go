package internal

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"testing"
)

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
