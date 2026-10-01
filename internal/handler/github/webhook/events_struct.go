package webhook

import "time"

type Commit struct {
	CommitSha   string    `json:"id"`
	Message     string    `json:"message"`
	Added       []string  `json:"added"`
	Removed     []string  `json:"removed"`
	Modified    []string  `json:"modified"`
	URL         string    `json:"url"`
	CommittedAt time.Time `json:"timestamp"`
}

type Owner struct {
	ID int64 `json:"id"`
}

type Repository struct {
	ID       int64  `json:"id"`
	FullName string `json:"full_name"`
	OwnerID  Owner  `json:"owner"`
}

type HeadCommit struct {
	SHA string `json:"id"`
}

type GitHubPush struct {
	RepoInfo     Repository  `json:"repository"`
	Ref          string      `json:"ref"`
	BeforeSHA    string      `json:"before"`
	AfterSHA     string      `json:"after"`
	HeadCommitID *HeadCommit `json:"head_commit"`
	Compare      string      `json:"compare"`
	Forced       bool        `json:"forced"`
	Created      bool        `json:"created"`
	Deleted      bool        `json:"deleted"`
	CommitInfo   []Commit    `json:"commits"`
}
