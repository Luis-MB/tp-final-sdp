package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	internalcrypto "tp-final-sdp/internal/crypto"
)

func TestNormalizeCreateJobRequestUsesPasswordLength(t *testing.T) {
	req := createJobRequest{
		Password:  "ab",
		Charset:   "ab",
		ChunkSize: 2,
	}

	normalizeCreateJobRequest(&req)

	if req.TargetHash != internalcrypto.SHA256Hex("ab") {
		t.Fatalf("TargetHash = %q, want SHA-256 of password", req.TargetHash)
	}
	if req.MinLength != 2 {
		t.Fatalf("MinLength = %d, want 2", req.MinLength)
	}
	if req.MaxLength != 2 {
		t.Fatalf("MaxLength = %d, want 2", req.MaxLength)
	}
}

func TestNormalizeCreateJobRequestUsesDefaultLengths(t *testing.T) {
	req := createJobRequest{
		TargetHash: "hash",
		Charset:    "abc",
		ChunkSize:  100,
	}

	normalizeCreateJobRequest(&req)

	if req.MinLength != defaultMinLength {
		t.Fatalf("MinLength = %d, want %d", req.MinLength, defaultMinLength)
	}
	if req.MaxLength != defaultMaxLength {
		t.Fatalf("MaxLength = %d, want %d", req.MaxLength, defaultMaxLength)
	}
}

func TestRequireTokenRejectsMissingToken(t *testing.T) {
	handler := requireToken("secret", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	recorder := httptest.NewRecorder()
	handler(recorder, httptest.NewRequest(http.MethodGet, "/jobs", nil))

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}

func TestRequireTokenAcceptsBearerToken(t *testing.T) {
	handler := requireToken("secret", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	request := httptest.NewRequest(http.MethodGet, "/jobs", nil)
	request.Header.Set("Authorization", "Bearer secret")

	recorder := httptest.NewRecorder()
	handler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
}
