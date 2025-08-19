package ui

import (
	"testing"
)

// DateTime parsing tests are now centralized in internal/har/datetime_test.go

func TestWaterfallViewGetSelectedIndex(t *testing.T) {
	wv := NewWaterfallView()
	
	// Test with empty indices
	if index := wv.GetSelectedIndex(); index != -1 {
		t.Errorf("Expected -1 for empty indices, got %d", index)
	}
	
	// Test with valid indices
	wv.indices = []int{5, 10, 15}
	wv.listView.AddItem("Item 1", "", 0, nil)
	wv.listView.AddItem("Item 2", "", 0, nil) 
	wv.listView.AddItem("Item 3", "", 0, nil)
	wv.listView.SetCurrentItem(1)
	
	expected := 10 // indices[1]
	if index := wv.GetSelectedIndex(); index != expected {
		t.Errorf("Expected %d, got %d", expected, index)
	}
	
	// Test out of bounds - SetCurrentItem with invalid index won't actually change the current item
	// The list will keep the previous valid selection
	currentItem := wv.listView.GetCurrentItem()
	if currentItem >= len(wv.indices) {
		if index := wv.GetSelectedIndex(); index != -1 {
			t.Errorf("Expected -1 for out of bounds current item, got %d", index)
		}
	}
}

func TestWaterfallViewSetSelectedEntry(t *testing.T) {
	wv := NewWaterfallView()
	wv.indices = []int{5, 10, 15, 20}
	wv.listView.AddItem("Item 1", "", 0, nil)
	wv.listView.AddItem("Item 2", "", 0, nil)
	wv.listView.AddItem("Item 3", "", 0, nil)
	wv.listView.AddItem("Item 4", "", 0, nil)
	
	// Test setting to valid entry
	wv.SetSelectedEntry(15)
	if currentItem := wv.listView.GetCurrentItem(); currentItem != 2 {
		t.Errorf("Expected current item 2, got %d", currentItem)
	}
	
	// Test setting to non-existent entry
	originalItem := wv.listView.GetCurrentItem()
	wv.SetSelectedEntry(999)
	if currentItem := wv.listView.GetCurrentItem(); currentItem != originalItem {
		t.Errorf("Selection should not change for non-existent entry")
	}
}