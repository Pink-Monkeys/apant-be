package queue

// ScanJob models async scan work payload consumed by worker interfaces.
type ScanJob struct {
	SessionID string
	Provider  string
	Model     string
	Message   string
	System    string
}
