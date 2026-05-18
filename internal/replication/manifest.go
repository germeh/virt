package replication

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
)

type DiskManifest struct {
	DiskID           string          `json:"disk_id"`
	Format           string          `json:"format"`
	VirtualSizeBytes uint64          `json:"virtual_size_bytes"`
	BlockSizeBytes   uint64          `json:"block_size_bytes"`
	Generation       uint64          `json:"generation"`
	Chunks           []ChunkManifest `json:"chunks"`
}

type ChunkManifest struct {
	Index      uint64     `json:"index"`
	Range      BlockRange `json:"range"`
	SHA256Hex  string     `json:"sha256_hex"`
	Verified   bool       `json:"verified"`
	RetryCount uint8      `json:"retry_count"`
}

func BuildChunkManifest(diskID string, format string, virtualSize uint64, chunkSize uint64, read func(BlockRange) ([]byte, error)) (DiskManifest, error) {
	if diskID == "" {
		return DiskManifest{}, errors.New("disk id is required")
	}
	if format == "" {
		return DiskManifest{}, errors.New("disk format is required")
	}
	if virtualSize == 0 {
		return DiskManifest{}, errors.New("virtual size must be greater than zero")
	}
	if chunkSize == 0 {
		return DiskManifest{}, errors.New("chunk size must be greater than zero")
	}
	if read == nil {
		return DiskManifest{}, errors.New("read function is required")
	}

	manifest := DiskManifest{
		DiskID:           diskID,
		Format:           format,
		VirtualSizeBytes: virtualSize,
		BlockSizeBytes:   chunkSize,
		Generation:       1,
	}
	for offset, index := uint64(0), uint64(0); offset < virtualSize; index++ {
		length := chunkSize
		if remaining := virtualSize - offset; remaining < chunkSize {
			length = remaining
		}
		r := BlockRange{Offset: offset, Length: length}
		data, err := read(r)
		if err != nil {
			return DiskManifest{}, fmt.Errorf("read chunk %d range offset %d length %d: %w", index, r.Offset, r.Length, err)
		}
		if uint64(len(data)) != r.Length {
			return DiskManifest{}, fmt.Errorf("short read for chunk %d range offset %d length %d: got %d bytes", index, r.Offset, r.Length, len(data))
		}
		manifest.Chunks = append(manifest.Chunks, ChunkManifest{
			Index:     index,
			Range:     r,
			SHA256Hex: ChecksumHex(data),
		})
		offset += length
	}
	return manifest, nil
}

func ChecksumHex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
