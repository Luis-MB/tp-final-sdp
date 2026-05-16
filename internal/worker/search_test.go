package worker

import (
	"testing"

	"tp-final-sdp/internal/crypto"
)

func TestSearchSHA256RangeFindsPlaintext(t *testing.T) {
	target := crypto.SHA256Hex("ba")

	result := SearchSHA256Range(target, "ab", 1, 2, Range{Start: 0, End: 6})

	if !result.Found {
		t.Fatal("expected plaintext to be found")
	}
	if result.Plaintext != "ba" {
		t.Fatalf("plaintext = %q, want %q", result.Plaintext, "ba")
	}
}

func TestSearchSHA256RangeParallelFindsPlaintext(t *testing.T) {
	target := crypto.SHA256Hex("ba")

	result := SearchSHA256RangeParallel(target, "ab", 1, 2, Range{Start: 0, End: 6}, 4)

	if !result.Found {
		t.Fatal("expected plaintext to be found")
	}
	if result.Plaintext != "ba" {
		t.Fatalf("plaintext = %q, want %q", result.Plaintext, "ba")
	}
}

func TestSearchSHA256RangeParallelHandlesMoreWorkersThanCandidates(t *testing.T) {
	target := crypto.SHA256Hex("b")

	result := SearchSHA256RangeParallel(target, "ab", 1, 1, Range{Start: 0, End: 2}, 8)

	if !result.Found {
		t.Fatal("expected plaintext to be found")
	}
	if result.Plaintext != "b" {
		t.Fatalf("plaintext = %q, want %q", result.Plaintext, "b")
	}
}
