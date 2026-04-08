package issuedocsync

import "time"

type Repo struct {
	Owner string
	Name  string
}

func (r Repo) String() string {
	return r.Owner + "/" + r.Name
}

type IssueSummary struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	UpdatedAt time.Time `json:"updatedAt"`
	ClosedAt  time.Time `json:"closedAt,omitempty"`
	URL       string    `json:"url,omitempty"`
}

type IssueRecord struct {
	Number     int      `json:"number"`
	Title      string   `json:"title,omitempty"`
	UpdatedAt  string   `json:"updatedAt"`
	Decision   string   `json:"decision,omitempty"`
	Reason     string   `json:"reason,omitempty"`
	TargetDocs []string `json:"targetDocs,omitempty"`
	SourceURL  string   `json:"sourceUrl,omitempty"`
	RecordedAt string   `json:"recordedAt,omitempty"`
}

type StateFile struct {
	Version int                    `json:"version"`
	Repo    string                 `json:"repo"`
	Issues  map[string]IssueRecord `json:"issues"`
}

type PlanCandidate struct {
	Number            int    `json:"number"`
	Title             string `json:"title"`
	UpdatedAt         string `json:"updatedAt"`
	URL               string `json:"url,omitempty"`
	Reason            string `json:"reason"`
	PreviousUpdatedAt string `json:"previousUpdatedAt,omitempty"`
}

type PlanReport struct {
	Repo             string          `json:"repo"`
	ScannedClosed    int             `json:"scannedClosed"`
	CandidateCount   int             `json:"candidateCount"`
	CachedIssueCount int             `json:"cachedIssueCount"`
	Candidates       []PlanCandidate `json:"candidates"`
}
