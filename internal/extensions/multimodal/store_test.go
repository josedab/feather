package multimodal

import (
	"testing"
)

func TestStoreAndGet(t *testing.T) {
	store := NewMultiModalStore(DefaultStoreConfig())
	data := []byte("hello world image data")

	meta, err := store.Store(ModalityImage, "image/png", data, map[string]string{"label": "test"})
	if err != nil {
		t.Fatalf("Store: %v", err)
	}
	if meta.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if meta.Modality != ModalityImage {
		t.Errorf("modality = %q, want %q", meta.Modality, ModalityImage)
	}
	if meta.OriginalSize != int64(len(data)) {
		t.Errorf("original_size = %d, want %d", meta.OriginalSize, len(data))
	}
	if meta.Hash == "" {
		t.Error("expected non-empty hash")
	}

	got, gotMeta, err := store.Get(meta.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("data mismatch: got %q, want %q", got, data)
	}
	if gotMeta.AccessCount != 1 {
		t.Errorf("access_count = %d, want 1", gotMeta.AccessCount)
	}
}

func TestGetNotFound(t *testing.T) {
	store := NewMultiModalStore(DefaultStoreConfig())
	_, _, err := store.Get("nonexistent")
	if err != ErrBlobNotFound {
		t.Errorf("err = %v, want ErrBlobNotFound", err)
	}
}

func TestDelete(t *testing.T) {
	store := NewMultiModalStore(DefaultStoreConfig())
	data := []byte("delete me")
	meta, _ := store.Store(ModalityText, "text/plain", data, nil)

	if err := store.Delete(meta.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, _, err := store.Get(meta.ID)
	if err != ErrBlobNotFound {
		t.Errorf("expected ErrBlobNotFound after delete, got %v", err)
	}
}

func TestDeduplication(t *testing.T) {
	store := NewMultiModalStore(DefaultStoreConfig())
	data := []byte("duplicate content")

	meta1, err := store.Store(ModalityText, "text/plain", data, nil)
	if err != nil {
		t.Fatalf("first Store: %v", err)
	}

	meta2, err := store.Store(ModalityText, "text/plain", data, nil)
	if err != nil {
		t.Fatalf("second Store: %v", err)
	}

	if meta1.ID != meta2.ID {
		t.Errorf("expected same ID for duplicate, got %q and %q", meta1.ID, meta2.ID)
	}

	stats := store.Stats()
	if stats.DuplicatesAvoided != 1 {
		t.Errorf("duplicates_avoided = %d, want 1", stats.DuplicatesAvoided)
	}
}

func TestCompression(t *testing.T) {
	store := NewMultiModalStore(DefaultStoreConfig())
	// Repetitive data compresses well
	data := make([]byte, 10000)
	for i := range data {
		data[i] = 'A'
	}

	meta, err := store.Store(ModalityDocument, "application/pdf", data, nil)
	if err != nil {
		t.Fatalf("Store: %v", err)
	}

	if meta.CompressedSize >= meta.OriginalSize {
		t.Errorf("compressed_size=%d should be less than original_size=%d for repetitive data",
			meta.CompressedSize, meta.OriginalSize)
	}

	got, _, err := store.Get(meta.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(got) != len(data) {
		t.Errorf("decompressed size %d != original %d", len(got), len(data))
	}
}

func TestNoCompression(t *testing.T) {
	cfg := DefaultStoreConfig()
	cfg.DefaultCompression = CompressionNone
	store := NewMultiModalStore(cfg)

	data := []byte("uncompressed data")
	meta, err := store.Store(ModalityText, "text/plain", data, nil)
	if err != nil {
		t.Fatalf("Store: %v", err)
	}
	if meta.CompressedSize != meta.OriginalSize {
		t.Errorf("with no compression, compressed=%d should equal original=%d",
			meta.CompressedSize, meta.OriginalSize)
	}
}

func TestBlobTooLarge(t *testing.T) {
	cfg := DefaultStoreConfig()
	cfg.MaxBlobSize = 100
	store := NewMultiModalStore(cfg)

	data := make([]byte, 200)
	_, err := store.Store(ModalityImage, "image/png", data, nil)
	if err == nil {
		t.Fatal("expected error for oversized blob")
	}
}

func TestList(t *testing.T) {
	store := NewMultiModalStore(DefaultStoreConfig())
	store.Store(ModalityImage, "image/png", []byte("img1"), nil)
	store.Store(ModalityImage, "image/jpeg", []byte("img2"), nil)
	store.Store(ModalityText, "text/plain", []byte("txt1"), nil)

	images := store.List(ModalityImage)
	if len(images) != 2 {
		t.Errorf("image count = %d, want 2", len(images))
	}

	all := store.List("")
	if len(all) != 3 {
		t.Errorf("total count = %d, want 3", len(all))
	}
}

func TestSearch(t *testing.T) {
	store := NewMultiModalStore(DefaultStoreConfig())
	store.Store(ModalityImage, "image/png", []byte("cat image"), map[string]string{"animal": "cat"})
	store.Store(ModalityImage, "image/png", []byte("dog image"), map[string]string{"animal": "dog"})
	store.Store(ModalityText, "text/plain", []byte("text about cats"), map[string]string{"topic": "feline"})

	results := store.Search("cat")
	if len(results) != 1 {
		t.Errorf("search for 'cat' returned %d results, want 1", len(results))
	}
}

func TestGetByHash(t *testing.T) {
	store := NewMultiModalStore(DefaultStoreConfig())
	data := []byte("hashable content")
	meta, _ := store.Store(ModalityText, "text/plain", data, nil)

	found, err := store.GetByHash(meta.Hash)
	if err != nil {
		t.Fatalf("GetByHash: %v", err)
	}
	if found.ID != meta.ID {
		t.Errorf("found.ID = %q, want %q", found.ID, meta.ID)
	}

	_, err = store.GetByHash("nonexistent")
	if err != ErrHashNotFound {
		t.Errorf("expected ErrHashNotFound, got %v", err)
	}
}

func TestStats(t *testing.T) {
	store := NewMultiModalStore(DefaultStoreConfig())
	store.Store(ModalityImage, "image/png", []byte("image data here"), nil)
	store.Store(ModalityAudio, "audio/wav", []byte("audio data here"), nil)

	stats := store.Stats()
	if stats.TotalBlobs != 2 {
		t.Errorf("total_blobs = %d, want 2", stats.TotalBlobs)
	}
	if stats.BlobsByModality["image"] != 1 || stats.BlobsByModality["audio"] != 1 {
		t.Errorf("unexpected blobs_by_modality: %v", stats.BlobsByModality)
	}
}

func TestGetMetadata(t *testing.T) {
	store := NewMultiModalStore(DefaultStoreConfig())
	data := []byte("metadata test")
	meta, _ := store.Store(ModalityDocument, "application/pdf", data, map[string]string{"key": "val"})

	got, err := store.GetMetadata(meta.ID)
	if err != nil {
		t.Fatalf("GetMetadata: %v", err)
	}
	if got.ContentType != "application/pdf" {
		t.Errorf("content_type = %q, want application/pdf", got.ContentType)
	}
}
