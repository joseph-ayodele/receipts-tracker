package constants

// JobStatus is the canonical status for rows in extract_job.
type JobStatus string

// Stable values (store these exact strings in DB).
const (
	JobStatusQueued  JobStatus = "QUEUED"   // optional: queued for processing
	JobStatusRunning JobStatus = "RUNNING"  // in progress
	JobStatusOCROK   JobStatus = "OCR_OK"   // stage 1 completed (text extracted)
	JobStatusLLMOK   JobStatus = "LLM_OK"   // stage 2 completed (fields extracted)
	JobStatusFailed  JobStatus = "FAILED"   // terminal failure
)
