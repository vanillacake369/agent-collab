package libp2p

import (
	"strings"
	"testing"
)

func TestContentStore_PutGet(t *testing.T) {
	cs := NewContentStore(DefaultContentStoreConfig())

	data := []byte("hello world")
	cid, err := cs.Put(data)
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	if !strings.HasPrefix(string(cid), "sha256-") {
		t.Errorf("CID should start with sha256-, got %s", cid)
	}

	retrieved, err := cs.Get(cid)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if string(retrieved) != string(data) {
		t.Errorf("Data mismatch: got %s, want %s", retrieved, data)
	}
}

func TestContentStore_Deduplication(t *testing.T) {
	cs := NewContentStore(DefaultContentStoreConfig())

	data := []byte("duplicate content")

	cid1, _ := cs.Put(data)
	cid2, _ := cs.Put(data)

	if cid1 != cid2 {
		t.Errorf("Same content should produce same CID: %s != %s", cid1, cid2)
	}

	stats := cs.Stats()
	if stats.ItemCount != 1 {
		t.Errorf("Should have 1 item, got %d", stats.ItemCount)
	}
}

func TestContentStore_Has(t *testing.T) {
	cs := NewContentStore(DefaultContentStoreConfig())

	data := []byte("test data")
	cid, _ := cs.Put(data)

	if !cs.Has(cid) {
		t.Error("Has should return true for existing content")
	}

	if cs.Has("nonexistent-cid") {
		t.Error("Has should return false for nonexistent content")
	}
}

func TestContentStore_Delete(t *testing.T) {
	cs := NewContentStore(DefaultContentStoreConfig())

	data := []byte("to be deleted")
	cid, _ := cs.Put(data)

	if !cs.Has(cid) {
		t.Fatal("Content should exist after Put")
	}

	err := cs.Delete(cid)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	if cs.Has(cid) {
		t.Error("Content should not exist after Delete")
	}
}

func TestContentStore_Eviction(t *testing.T) {
	config := ContentStoreConfig{
		MaxSize: 100, // 100 bytes only
	}
	cs := NewContentStore(config)

	// Fill with small items
	for i := 0; i < 5; i++ {
		data := make([]byte, 30)
		for j := range data {
			data[j] = byte(i)
		}
		_, _ = cs.Put(data)
	}

	// Should have evicted some items
	stats := cs.Stats()
	if stats.TotalSize > config.MaxSize {
		t.Errorf("Total size %d exceeds max size %d", stats.TotalSize, config.MaxSize)
	}
}

func TestContentStore_Metadata(t *testing.T) {
	cs := NewContentStore(DefaultContentStoreConfig())

	data := []byte("content with metadata")
	cid, _ := cs.PutWithMimeType(data, "text/plain")

	meta := cs.GetMetadata(cid)
	if meta == nil {
		t.Fatal("Metadata should exist")
	}

	if meta.Size != len(data) {
		t.Errorf("Size mismatch: got %d, want %d", meta.Size, len(data))
	}

	if meta.MimeType != "text/plain" {
		t.Errorf("MimeType mismatch: got %s, want text/plain", meta.MimeType)
	}
}

func TestContentStore_CreateReference(t *testing.T) {
	cs := NewContentStore(DefaultContentStoreConfig())

	data := []byte("referenced content")
	cid, _ := cs.PutWithMimeType(data, "application/json")

	ref := cs.CreateReference(cid, "node1")
	if ref == nil {
		t.Fatal("Reference should not be nil")
	}

	if ref.CID != cid {
		t.Errorf("CID mismatch: got %s, want %s", ref.CID, cid)
	}

	if ref.Size != len(data) {
		t.Errorf("Size mismatch: got %d, want %d", ref.Size, len(data))
	}

	if ref.CreatedBy != "node1" {
		t.Errorf("CreatedBy mismatch: got %s, want node1", ref.CreatedBy)
	}
}

func TestContentStore_List(t *testing.T) {
	cs := NewContentStore(DefaultContentStoreConfig())

	// Add some content
	cs.Put([]byte("content1"))
	cs.Put([]byte("content2"))
	cs.Put([]byte("content3"))

	list := cs.List()
	if len(list) != 3 {
		t.Errorf("Expected 3 items, got %d", len(list))
	}
}

func TestValidateCID(t *testing.T) {
	data := []byte("validate me")
	cid := computeCID(data)

	if !ValidateCID(cid, data) {
		t.Error("ValidateCID should return true for matching data")
	}

	if ValidateCID(cid, []byte("different data")) {
		t.Error("ValidateCID should return false for different data")
	}
}

func TestContentStore_WrapUnwrapInline(t *testing.T) {
	cs := NewContentStore(DefaultContentStoreConfig())

	// Small content should be inlined
	smallData := []byte("small content")
	msg, err := cs.WrapContent(smallData, "node1")
	if err != nil {
		t.Fatalf("WrapContent failed: %v", err)
	}

	if msg.Type != "inline" {
		t.Errorf("Small content should be inlined, got type %s", msg.Type)
	}

	unwrapped, err := cs.UnwrapContent(msg)
	if err != nil {
		t.Fatalf("UnwrapContent failed: %v", err)
	}

	if string(unwrapped) != string(smallData) {
		t.Errorf("Data mismatch after unwrap")
	}
}

func TestContentStore_WrapUnwrapReference(t *testing.T) {
	cs := NewContentStore(DefaultContentStoreConfig())

	// Large content should be referenced
	largeData := make([]byte, ContentThreshold+1000)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	msg, err := cs.WrapContent(largeData, "node1")
	if err != nil {
		t.Fatalf("WrapContent failed: %v", err)
	}

	if msg.Type != "reference" {
		t.Errorf("Large content should be referenced, got type %s", msg.Type)
	}

	if msg.Reference == nil {
		t.Fatal("Reference should not be nil")
	}

	unwrapped, err := cs.UnwrapContent(msg)
	if err != nil {
		t.Fatalf("UnwrapContent failed: %v", err)
	}

	if len(unwrapped) != len(largeData) {
		t.Errorf("Data length mismatch: got %d, want %d", len(unwrapped), len(largeData))
	}
}

func TestContentStore_Stats(t *testing.T) {
	config := ContentStoreConfig{
		MaxSize: 1000,
	}
	cs := NewContentStore(config)

	cs.Put([]byte("hello"))
	cs.Put([]byte("world"))

	stats := cs.Stats()

	if stats.ItemCount != 2 {
		t.Errorf("Expected 2 items, got %d", stats.ItemCount)
	}

	if stats.TotalSize != 10 { // "hello" + "world"
		t.Errorf("Expected 10 bytes, got %d", stats.TotalSize)
	}

	if stats.MaxSize != 1000 {
		t.Errorf("MaxSize mismatch: got %d", stats.MaxSize)
	}

	expectedRatio := float64(10) / float64(1000)
	if stats.UsageRatio != expectedRatio {
		t.Errorf("UsageRatio mismatch: got %f, want %f", stats.UsageRatio, expectedRatio)
	}
}

func BenchmarkContentStore_Put(b *testing.B) {
	cs := NewContentStore(DefaultContentStoreConfig())
	data := make([]byte, 1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data[0] = byte(i)
		_, _ = cs.Put(data)
	}
}

func BenchmarkContentStore_Get(b *testing.B) {
	cs := NewContentStore(DefaultContentStoreConfig())
	data := []byte("benchmark data")
	cid, _ := cs.Put(data)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = cs.Get(cid)
	}
}
