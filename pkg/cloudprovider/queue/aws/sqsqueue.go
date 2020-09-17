package aws

type SQSQueue struct {
	ARN string
}

func (q *SQSQueue) Name() string {
	return q.ARN
}

// Length returns the length of the queue
func (q *SQSQueue) Length() (int64, error) {
	return 0, nil
}

// OldestMessageAge returns the age of the oldest message
func (q *SQSQueue) OldestMessageAge() (int64, error) {
	return 0, nil
}
