package libp2p

import (
	"encoding/binary"
	"fmt"

	"github.com/klauspost/compress/zstd"
)

// CompressionType defines the compression algorithm used
type CompressionType byte

const (
	CompressionNone CompressionType = 0x00
	CompressionZstd CompressionType = 0x01
)

// compressionThreshold is the minimum size for compression (1KB)
const compressionThreshold = 1024

// compressionRatio is the minimum compression ratio to apply compression (20% reduction)
const compressionRatio = 0.8

var (
	encoder *zstd.Encoder
	decoder *zstd.Decoder
)

func init() {
	var err error
	encoder, err = zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedDefault))
	if err != nil {
		panic(fmt.Sprintf("failed to create zstd encoder: %v", err))
	}
	decoder, err = zstd.NewReader(nil)
	if err != nil {
		panic(fmt.Sprintf("failed to create zstd decoder: %v", err))
	}
}

// CompressedMessage wraps a message with compression metadata
// Wire format: [1 byte type][4 bytes original size][compressed data]
type CompressedMessage struct {
	Type         CompressionType
	OriginalSize uint32
	Data         []byte
}

// CompressMessage compresses data if it's large enough and compression is beneficial
func CompressMessage(data []byte) []byte {
	// Don't compress small messages
	if len(data) < compressionThreshold {
		return wrapUncompressed(data)
	}

	// Try to compress
	compressed := encoder.EncodeAll(data, nil)

	// Only use compression if it reduces size by at least 20%
	if float64(len(compressed)) < float64(len(data))*compressionRatio {
		return wrapCompressed(compressed, len(data))
	}

	return wrapUncompressed(data)
}

// DecompressMessage decompresses data if it was compressed
func DecompressMessage(data []byte) ([]byte, error) {
	if len(data) < 5 {
		return nil, fmt.Errorf("message too short: %d bytes", len(data))
	}

	compressionType := CompressionType(data[0])
	originalSize := binary.BigEndian.Uint32(data[1:5])
	payload := data[5:]

	switch compressionType {
	case CompressionNone:
		if uint32(len(payload)) != originalSize {
			return nil, fmt.Errorf("size mismatch: expected %d, got %d", originalSize, len(payload))
		}
		return payload, nil

	case CompressionZstd:
		decompressed, err := decoder.DecodeAll(payload, nil)
		if err != nil {
			return nil, fmt.Errorf("zstd decompression failed: %w", err)
		}
		if uint32(len(decompressed)) != originalSize {
			return nil, fmt.Errorf("decompressed size mismatch: expected %d, got %d", originalSize, len(decompressed))
		}
		return decompressed, nil

	default:
		return nil, fmt.Errorf("unknown compression type: %d", compressionType)
	}
}

// wrapUncompressed wraps data without compression
func wrapUncompressed(data []byte) []byte {
	result := make([]byte, 5+len(data))
	result[0] = byte(CompressionNone)
	binary.BigEndian.PutUint32(result[1:5], uint32(len(data)))
	copy(result[5:], data)
	return result
}

// wrapCompressed wraps compressed data with metadata
func wrapCompressed(compressed []byte, originalSize int) []byte {
	result := make([]byte, 5+len(compressed))
	result[0] = byte(CompressionZstd)
	binary.BigEndian.PutUint32(result[1:5], uint32(originalSize))
	copy(result[5:], compressed)
	return result
}

// IsCompressedMessage checks if the data has a valid compression header
func IsCompressedMessage(data []byte) bool {
	if len(data) < 5 {
		return false
	}
	compressionType := CompressionType(data[0])
	return compressionType == CompressionNone || compressionType == CompressionZstd
}
