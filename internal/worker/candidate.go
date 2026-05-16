package worker

import "fmt"

func CandidateAt(index uint64, charset string, minLength, maxLength uint32) (string, error) {
	base := uint64(len(charset))
	if base == 0 {
		return "", fmt.Errorf("charset must not be empty")
	}
	if minLength == 0 || maxLength < minLength {
		return "", fmt.Errorf("invalid length range")
	}

	remaining := index
	for length := minLength; length <= maxLength; length++ {
		count, ok := pow(base, length)
		if !ok {
			return "", fmt.Errorf("candidate space overflows uint64")
		}
		if remaining < count {
			return candidateForLength(remaining, charset, length), nil
		}
		remaining -= count
	}

	return "", fmt.Errorf("candidate index out of range")
}

func TotalCandidates(charset string, minLength, maxLength uint32) (uint64, error) {
	base := uint64(len(charset))
	if base == 0 {
		return 0, fmt.Errorf("charset must not be empty")
	}
	if minLength == 0 || maxLength < minLength {
		return 0, fmt.Errorf("invalid length range")
	}

	var total uint64
	for length := minLength; length <= maxLength; length++ {
		count, ok := pow(base, length)
		if !ok || total > ^uint64(0)-count {
			return 0, fmt.Errorf("candidate space overflows uint64")
		}
		total += count
	}
	return total, nil
}

func candidateForLength(offset uint64, charset string, length uint32) string {
	base := uint64(len(charset))
	out := make([]byte, length)
	for i := int(length) - 1; i >= 0; i-- {
		out[i] = charset[offset%base]
		offset /= base
	}
	return string(out)
}

func pow(base uint64, exp uint32) (uint64, bool) {
	result := uint64(1)
	for range exp {
		if base != 0 && result > ^uint64(0)/base {
			return 0, false
		}
		result *= base
	}
	return result, true
}
