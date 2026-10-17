package webhook

import (
	"time"
)

// ==============================
// GENERAL HELPER (EVERYTHING BELOW)
// ==============================

type Repository struct {
	ID    int64 `json:"id"`
	Owner struct {
		ID int64 `json:"id"`
	} `json:"owner"`
}

// ==============================
// PULL REQUEST WEBHOOK ONLY (EVERYTHING BELOW)
// ==============================

type PullRequestWebhook struct {
	Action      string `json:"action"`
	PullRequest struct {
		Additions    int        `json:"additions"`
		Body         *string    `json:"body"`
		ChangedFiles int        `json:"changed_files"`
		ClosedAt     *time.Time `json:"closed_at"`
		CreatedAt    time.Time  `json:"created_at"`
		Deletions    int        `json:"deletions"`
		IsDraft      bool       `json:"draft"`
		Id           int64      `json:"id"`
		Labels       []struct {
			Name string `json:"name"`
		} `json:"labels"`
		IsLocked  bool       `json:"locked"`
		IsMerged  bool       `json:"merged"`
		MergedAt  *time.Time `json:"merged_at"`
		Number    int        `json:"number"`
		State     string     `json:"state"`
		Title     string     `json:"title"`
		UpdatedAt time.Time  `json:"updated_at"`
		HtmlUrl   string     `json:"html_url"`
		Author    *struct {
			Login string `json:"login"`
		} `json:"user"`
	}
	Repository Repository `json:"repository"`
}

// ==============================
// ISSUE WEBHOOK ONLY (EVERYTHING BELOW)
// ==============================

type IssuesWebhook struct {
	Action string `json:"action"`
	Issue  struct {
		Body      *string    `json:"body"`
		ClosedAt  *time.Time `json:"closed_at"`
		CreatedAt time.Time  `json:"createdAt"`
		HtmlUrl   string     `json:"html_url"`
		Id        int64      `json:"id"`
		Labels    []struct {
			Name string `json:"name"`
		} `json:"labels"`
		IsLocked  bool `json:"locked"`
		Number    int  `json:"number"`
		Reactions struct {
			Positive   int `json:"+1"`
			Negative   int `json:"-1"`
			Confused   int `json:"confused"`
			Eyes       int `json:"eyes"`
			Heart      int `json:"heart"`
			Hooray     int `json:"hooray"`
			Laugh      int `json:"laugh"`
			Rocket     int `json:"rocket"`
			TotalCount int `json:"total_count"`
		} `json:"reactions"`
		State       string  `json:"string"`
		StateReason *string `json:"state_reason"`
		Title       string  `json:"title"`
		Type        *struct {
			Name string `json:"name"`
		} `json:"type"`
		UpdatedAt time.Time `json:"updated_at"`
		Author    *struct {
			Login string `json:"login"`
		} `json:"user"`
	} `json:"issue"`
	Repository Repository `json:"repository"`
}

// ==============================
// RELEASE WEBHOOK ONLY (EVERYTHING BELOW)
// ==============================

type ReleasesWebhook struct {
	Action  string `json:"action"`
	Release struct {
		CreatedAt *time.Time `json:"created_at"`
		UpdatedAt *time.Time `json:"updated_at"`
		IsDraft   bool       `json:"draft"`
		HtmlUrl   string     `json:"html_url"`
		Id        int64      `json:"id"`
		Name      *string    `json:"name"`
		Author    *struct {
			Login string `json:"login"`
		} `json:"author"`
		TagName       string  `json:"tag_name"`
		IsImmutable   bool    `json:"immutable"`
		IsPrereleased bool    `json:"prerelease"`
		PublishedAt   *string `json:"published_at"`
		Description   *string `json:"body"`
	} `json:"release"`
	Repository Repository `json:"repository"`
}

// ==============================
// RELEASE WEBHOOK ONLY (EVERYTHING BELOW)
// ==============================
type IssueCommentsWebhook struct {
	Action       string `json:"action"`
	IssueComment struct {
		ID      int64  `json:"id"`
		Body    string `json:"body"`
		HtmlUrl string `json:"html_url"`
		Author  struct {
			Login string `json:"login"`
		} `json:"user"`
		IsMinimized *bool     `json:"minimized"`
		CreatedAt   time.Time `json:"created_at"`
		UpdatedAt   time.Time `json:"updated_at"`
	} `json:"comment"`
	Issue struct {
		ID int64 `json:"id"`
	} `json:"issue"`
}

// ==============================
// PUSH WEBHOOK ONLY (EVERYTHING BELOW)
// ==============================

type GitHubPush struct {
	RepoInfo     Repository `json:"repository"`
	Ref          string     `json:"ref"`
	BeforeSHA    string     `json:"before"`
	AfterSHA     string     `json:"after"`
	HeadCommitID *struct {
		SHA string `json:"id"`
	} `json:"head_commit,omitempty"`
	Compare    string `json:"compare"`
	Forced     bool   `json:"forced"`
	Created    bool   `json:"created"`
	Deleted    bool   `json:"deleted"`
	CommitInfo []struct {
		CommitSha   string    `json:"id"`
		Message     string    `json:"message"`
		Added       []string  `json:"added"`
		Removed     []string  `json:"removed"`
		Modified    []string  `json:"modified"`
		CommittedAt time.Time `json:"timestamp"`
	} `json:"commits"`
}

// ==============================
// REPOSITORY WEBHOOK ONLY (EVERYTHING BELOW)
// ==============================

type RepositoryWebhook struct {
	Action      string `json:"action"`
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	LicenseInfo *struct {
		Name    string  `json:"name"`
		Body    string  `json:"body"`
		Key     string  `json:"key"`
		SpdxId  *string `json:"spdxId"`
		HtmlUrl *string `json:"html_url"`
		NodeId  string  `json:"id"`
	} `json:"license"`
	ForksCount int `json:"forks_count"`
	Owner      struct {
		ID int64 `json:"id"`
	} `json:"owner"`
	IsPrivate   bool       `json:"private"`
	HtmlUrl     string     `json:"html_url"`
	Description *string    `json:"description"`
	IsFork      bool       `json:"fork"`
	Language    string     `json:"language"`
	Stars       int        `json:"stargazers_count"`
	Watches     int        `json:"watchers_count"`
	Topics      []string   `json:"topics"`
	IsArchived  bool       `json:"archived"`
	Visibility  *string    `json:"visibility"`
	PushedAt    *time.Time `json:"pushed_at"`
	CreatedAt   *time.Time `json:"created_at"`
	UpdatedAt   *time.Time `json:"updated_at"`
}
