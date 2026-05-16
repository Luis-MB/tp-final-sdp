package worker

import (
	"context"
	"sync"

	"tp-final-sdp/internal/crypto"
)

type Range struct {
	Start uint64
	End   uint64
}

type Result struct {
	Found     bool
	Plaintext string
}

func SearchSHA256(targetHash string, candidates []string) Result {
	for _, candidate := range candidates {
		if crypto.SHA256Hex(candidate) == targetHash {
			return Result{Found: true, Plaintext: candidate}
		}
	}
	return Result{}
}

func SearchSHA256Range(targetHash, charset string, minLength, maxLength uint32, searchRange Range) Result {
	for index := searchRange.Start; index < searchRange.End; index++ {
		candidate, err := CandidateAt(index, charset, minLength, maxLength)
		if err != nil {
			return Result{}
		}
		if crypto.SHA256Hex(candidate) == targetHash {
			return Result{Found: true, Plaintext: candidate}
		}
	}
	return Result{}
}

func SearchSHA256RangeParallel(targetHash, charset string, minLength, maxLength uint32, searchRange Range, concurrency int) Result {
	total := searchRange.End - searchRange.Start
	if concurrency <= 1 || total <= 1 {
		return SearchSHA256Range(targetHash, charset, minLength, maxLength, searchRange)
	}
	if uint64(concurrency) > total {
		concurrency = int(total)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resultCh := make(chan Result, 1)
	var wg sync.WaitGroup

	chunkSize := total / uint64(concurrency)
	remainder := total % uint64(concurrency)
	start := searchRange.Start

	for workerIndex := 0; workerIndex < concurrency; workerIndex++ {
		size := chunkSize
		if uint64(workerIndex) < remainder {
			size++
		}
		end := start + size

		wg.Add(1)
		go func(subRange Range) {
			defer wg.Done()
			for index := subRange.Start; index < subRange.End; index++ {
				select {
				case <-ctx.Done():
					return
				default:
				}

				candidate, err := CandidateAt(index, charset, minLength, maxLength)
				if err != nil {
					return
				}
				if crypto.SHA256Hex(candidate) == targetHash {
					select {
					case resultCh <- Result{Found: true, Plaintext: candidate}:
						cancel()
					default:
					}
					return
				}
			}
		}(Range{Start: start, End: end})

		start = end
	}

	doneCh := make(chan struct{})
	go func() {
		wg.Wait()
		close(doneCh)
	}()

	select {
	case result := <-resultCh:
		return result
	case <-doneCh:
		return Result{}
	}
}
