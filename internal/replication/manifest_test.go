package replication

import (
	"errors"
	"strings"
	"testing"
)

func TestBuildChunkManifestSplitsDiskIntoChunks(t *testing.T) {
	manifest, err := BuildChunkManifest("disk-1", "qcow2", 10, 4, func(r BlockRange) ([]byte, error) {
		return []byte("abcdefghij")[r.Offset:r.End()], nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(manifest.Chunks) != 3 {
		t.Fatalf("got %d chunks, want 3", len(manifest.Chunks))
	}
	if manifest.Chunks[2].Range != (BlockRange{Offset: 8, Length: 2}) {
		t.Fatalf("last chunk = %#v", manifest.Chunks[2].Range)
	}
}

func TestChecksumHexIsStable(t *testing.T) {
	got := ChecksumHex([]byte("abc"))
	want := "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestBuildChunkManifestRejectsShortRead(t *testing.T) {
	_, err := BuildChunkManifest("disk-1", "qcow2", 8, 4, func(BlockRange) ([]byte, error) {
		return nil, nil
	})
	if err == nil {
		t.Fatal("expected short read error")
	}
	if !strings.Contains(err.Error(), "chunk 0") {
		t.Fatalf("error %q does not include chunk index", err.Error())
	}
	if !strings.Contains(err.Error(), "offset 0 length 4") {
		t.Fatalf("error %q does not include range", err.Error())
	}
}

func TestBuildChunkManifestRequiresReader(t *testing.T) {
	_, err := BuildChunkManifest("disk-1", "qcow2", 8, 4, nil)
	if err == nil {
		t.Fatal("expected nil reader error")
	}
	if err.Error() != "read function is required" {
		t.Fatalf("got %q", err.Error())
	}
}

func TestBuildChunkManifestWrapsReaderErrorWithChunkContext(t *testing.T) {
	readErr := errors.New("read failed")
	_, err := BuildChunkManifest("disk-1", "qcow2", 8, 4, func(BlockRange) ([]byte, error) {
		return nil, readErr
	})
	if err == nil {
		t.Fatal("expected reader error")
	}
	if !errors.Is(err, readErr) {
		t.Fatalf("expected wrapped reader error, got %v", err)
	}
	if !strings.Contains(err.Error(), "chunk 0") {
		t.Fatalf("error %q does not include chunk index", err.Error())
	}
	if !strings.Contains(err.Error(), "offset 0 length 4") {
		t.Fatalf("error %q does not include range", err.Error())
	}
}

func TestBuildChunkManifestExactChunkBoundary(t *testing.T) {
	manifest, err := BuildChunkManifest("disk-1", "qcow2", 8, 4, func(r BlockRange) ([]byte, error) {
		return make([]byte, int(r.Length)), nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(manifest.Chunks) != 2 {
		t.Fatalf("got %d chunks, want 2", len(manifest.Chunks))
	}
	if manifest.Chunks[1].Range != (BlockRange{Offset: 4, Length: 4}) {
		t.Fatalf("last chunk = %#v", manifest.Chunks[1].Range)
	}
}

func TestBuildChunkManifestAdvancesByFinalChunkLength(t *testing.T) {
	var ranges []BlockRange
	manifest, err := BuildChunkManifest("disk-1", "qcow2", 10, 6, func(r BlockRange) ([]byte, error) {
		ranges = append(ranges, r)
		return make([]byte, int(r.Length)), nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(manifest.Chunks) != 2 {
		t.Fatalf("got %d chunks, want 2", len(manifest.Chunks))
	}
	if ranges[1] != (BlockRange{Offset: 6, Length: 4}) {
		t.Fatalf("second read range = %#v", ranges[1])
	}
}
