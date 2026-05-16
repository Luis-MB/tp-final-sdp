package worker

import "testing"

func TestCandidateAtUsesVariableLengths(t *testing.T) {
	tests := []struct {
		name      string
		index     uint64
		charset   string
		minLength uint32
		maxLength uint32
		want      string
	}{
		{name: "first length one", index: 0, charset: "ab", minLength: 1, maxLength: 3, want: "a"},
		{name: "second length one", index: 1, charset: "ab", minLength: 1, maxLength: 3, want: "b"},
		{name: "first length two", index: 2, charset: "ab", minLength: 1, maxLength: 3, want: "aa"},
		{name: "last length two", index: 5, charset: "ab", minLength: 1, maxLength: 3, want: "bb"},
		{name: "first length three", index: 6, charset: "ab", minLength: 1, maxLength: 3, want: "aaa"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := CandidateAt(test.index, test.charset, test.minLength, test.maxLength)
			if err != nil {
				t.Fatalf("CandidateAt() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("CandidateAt() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestTotalCandidates(t *testing.T) {
	got, err := TotalCandidates("abc", 1, 3)
	if err != nil {
		t.Fatalf("TotalCandidates() error = %v", err)
	}
	if got != 39 {
		t.Fatalf("TotalCandidates() = %d, want 39", got)
	}
}
