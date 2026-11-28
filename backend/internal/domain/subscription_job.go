package domain

type SubscriptionJob struct {
	ID        uint64
	Kind      string
	SubjectID string
	Payload   []byte
}

type SubscriptionJobSummary struct {
	ID        uint64 `json:"id"`
	Kind      string `json:"kind"`
	SubjectID string `json:"subjectId"`
	Result    string `json:"result"`
	Attempts  int    `json:"attempts"`
	LastError string `json:"lastError"`
}

// SubscriptionNotSent marks failures for which the message send was not accepted.
type SubscriptionNotSent struct {
	Cause         error
	InvalidGrant  bool
	Configuration bool
}

func (e *SubscriptionNotSent) Error() string { return e.Cause.Error() }
func (e *SubscriptionNotSent) Unwrap() error { return e.Cause }
