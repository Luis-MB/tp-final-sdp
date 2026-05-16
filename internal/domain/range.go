package domain

type RangeStatus string

const (
	RangeStatusPending   RangeStatus = "pending"
	RangeStatusLeased    RangeStatus = "leased"
	RangeStatusCompleted RangeStatus = "completed"
	RangeStatusFailed    RangeStatus = "failed"
)

type SearchRange struct {
	JobID  string
	Start  uint64
	End    uint64
	Status RangeStatus
}

