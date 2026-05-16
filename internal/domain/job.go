package domain

type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusFound     JobStatus = "found"
	JobStatusExhausted JobStatus = "exhausted"
	JobStatusCancelled JobStatus = "cancelled"
)

type Job struct {
	ID         string
	TargetHash string
	Charset    string
	MinLength  int
	MaxLength  int
	Status     JobStatus
}

