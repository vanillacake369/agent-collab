package libp2p

import (
	"bytes"
	"strings"
	"testing"
)

func TestCompressMessage_SmallData(t *testing.T) {
	// Small data should not be compressed
	data := []byte("hello world")
	compressed := CompressMessage(data)

	// Check header
	if compressed[0] != byte(CompressionNone) {
		t.Errorf("Expected no compression for small data, got type %d", compressed[0])
	}

	// Decompress and verify
	decompressed, err := DecompressMessage(compressed)
	if err != nil {
		t.Fatalf("Decompression failed: %v", err)
	}
	if !bytes.Equal(data, decompressed) {
		t.Errorf("Data mismatch: expected %s, got %s", data, decompressed)
	}
}

func TestCompressMessage_LargeData(t *testing.T) {
	// Large repetitive data should be compressed
	data := []byte(strings.Repeat("hello world ", 200)) // ~2.4KB of repetitive data

	compressed := CompressMessage(data)

	// Check that compression was applied
	if compressed[0] != byte(CompressionZstd) {
		t.Errorf("Expected zstd compression for large repetitive data, got type %d", compressed[0])
	}

	// Check compression ratio
	ratio := float64(len(compressed)) / float64(len(data))
	if ratio > 0.5 {
		t.Errorf("Expected significant compression, got ratio %.2f", ratio)
	}

	// Decompress and verify
	decompressed, err := DecompressMessage(compressed)
	if err != nil {
		t.Fatalf("Decompression failed: %v", err)
	}
	if !bytes.Equal(data, decompressed) {
		t.Errorf("Data mismatch after decompression")
	}
}

func TestCompressMessage_LargeIncompressibleData(t *testing.T) {
	// Large random-like data that doesn't compress well
	data := make([]byte, 2000)
	for i := range data {
		data[i] = byte(i % 256)
	}

	compressed := CompressMessage(data)

	// If compression doesn't help, it should fall back to uncompressed
	// (the exact behavior depends on the data)

	// Decompress and verify
	decompressed, err := DecompressMessage(compressed)
	if err != nil {
		t.Fatalf("Decompression failed: %v", err)
	}
	if !bytes.Equal(data, decompressed) {
		t.Errorf("Data mismatch after decompression")
	}
}

func TestDecompressMessage_InvalidData(t *testing.T) {
	// Too short
	_, err := DecompressMessage([]byte{0x00})
	if err == nil {
		t.Error("Expected error for too short message")
	}

	// Invalid compression type
	_, err = DecompressMessage([]byte{0xFF, 0x00, 0x00, 0x00, 0x05, 'h', 'e', 'l', 'l', 'o'})
	if err == nil {
		t.Error("Expected error for invalid compression type")
	}
}

func TestIsCompressedMessage(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected bool
	}{
		{"empty", []byte{}, false},
		{"too short", []byte{0x00, 0x01, 0x02}, false},
		{"valid none", []byte{0x00, 0x00, 0x00, 0x00, 0x05, 'h'}, true},
		{"valid zstd", []byte{0x01, 0x00, 0x00, 0x00, 0x05, 'h'}, true},
		{"invalid type", []byte{0x02, 0x00, 0x00, 0x00, 0x05, 'h'}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsCompressedMessage(tt.data)
			if result != tt.expected {
				t.Errorf("IsCompressedMessage(%v) = %v, want %v", tt.data, result, tt.expected)
			}
		})
	}
}

func BenchmarkCompressMessage(b *testing.B) {
	data := []byte(strings.Repeat(`{"type":"shared_context","content":"test content","file_path":"test.go"}`, 50))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CompressMessage(data)
	}
}

func BenchmarkDecompressMessage(b *testing.B) {
	data := []byte(strings.Repeat(`{"type":"shared_context","content":"test content","file_path":"test.go"}`, 50))
	compressed := CompressMessage(data)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = DecompressMessage(compressed)
	}
}
