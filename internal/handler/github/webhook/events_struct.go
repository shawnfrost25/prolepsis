package webhook

import (
	"time"
)

// ==============================
// ISSUE/PR COMMENTS
// ==============================

type Comments struct {
	Nodes []struct {
		Author *struct {
			Login string `json:"login"`
		} `json:"author"`
		Body            string     `json:"body"`
		FullDatabaseId  *string    `json:"fullDatabaseId"`
		IsMinimized     bool       `json:"isMinimized"`
		MinimizedReason *string    `json:"minimizedReason"`
		CreatedAt       time.Time  `json:"createdAt"`
		UpdatedAt       time.Time  `json:"updatedAt"`
		LastEditedAt    *time.Time `json:"lastEditedAt"`
		HtmlUrl         string     `json:"url"`
	} `json:"nodes"`
}

// ==============================
// PULL REQUEST
// ==============================

type PullRequestNode struct {
	Additions int `json:"additions"`
	Author    *struct {
		Login string `json:"login"`
	} `json:"author"`
	Body           string   `json:"body"`
	ChangedFiles   int      `json:"changedFiles"`
	Comments       Comments `json:"comments"`
	FullDatabaseId *string  `json:"fullDatabaseId"`
	Deletions      int      `json:"deletions"`
	IsDraft        bool     `json:"isDraft"`
	IsLocked       bool     `json:"locked"`
	IsMerged       bool     `json:"merged"`
	IsClosed       bool     `json:"closed"`
	Number         int      `json:"number"`
	ReactionGroups []struct {
		Content  string `json:"content"`
		Reactors struct {
			TotalCount int `json:"totalCount"`
		} `json:"reactors"`
	} `json:"reactionGroups"`
	Participants struct {
		TotalCount int `json:"totalCount"`
	} `json:"participants"`
	Labels struct {
		Nodes []struct {
			Name string `json:"name"`
		} `json:"nodes"`
	} `json:"labels"`
	State     string     `json:"state"`
	HtmlUrl   string     `json:"url"`
	Title     string     `json:"title"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	ClosedAt  *time.Time `json:"closedAt"`
	MergedAt  *time.Time `json:"mergedAt"`
}

type PullRequest struct {
	Nodes []PullRequestNode `json:"nodes"`
}

// ==============================
// ISSUES
// ==============================
type IssueNode struct {
	Author *struct {
		Login string `json:"login"`
	} `json:"author"`
	Body           string     `json:"body"`
	Comments       Comments   `json:"comments"`
	IsClosed       bool       `json:"closed"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
	ClosedAt       *time.Time `json:"closedAt"`
	FullDatabaseId *string    `json:"fullDatabaseId"`
	Labels         struct {
		Nodes []struct {
			Name string `json:"name"`
		} `json:"nodes"`
	} `json:"labels"`
	IssueType *struct {
		Name string `json:"name"`
	} `json:"issueType"`
	LastEditedAt   *time.Time `json:"lastEditedAt"`
	IsLocked       bool       `json:"locked"`
	Number         int        `json:"number"`
	ReactionGroups []struct {
		Content  string `json:"content"`
		Reactors struct {
			TotalCount int `json:"totalCount"`
		} `json:"reactors"`
	} `json:"reactionGroups"`
	Participants struct {
		TotalCount int `json:"totalCount"`
	} `json:"participants"`
	State       string  `json:"state"`
	StateReason *string `json:"stateReason"`
	Title       string  `json:"title"`
	HtmlUrl     string  `json:"url"`
}

type Issues struct {
	Nodes []IssueNode `json:"nodes"`
}

// ==============================
// RELEASES
// ==============================

type ReleaseNode struct {
	DatabaseId *int `json:"databaseId"`
	Author     *struct {
		Login string `json:"login"`
	} `json:"author"`
	Description   *string    `json:"description"`
	IsImmutable   bool       `json:"immutable"`
	IsPrereleased bool       `json:"isPrerelease"`
	IsDraft       bool       `json:"isDraft"`
	HtmlUrl       string     `json:"url"`
	TagName       string     `json:"tagName"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
	PublishedAt   *time.Time `json:"publishedAt"`
}

type Releases struct {
	TotalCount int           `json:"totalCount"`
	Nodes      []ReleaseNode `json:"nodes"`
}

// ==============================
// REPOSITORY
// ==============================

type Repository struct {
	TotalCount int `json:"totalCount"`
	// Thought to write it as *[]struct - but it's useless and adds more unwanted abstarction
	Nodes []struct {
		Name            string    `json:"name"`
		IsArchived      bool      `json:"isArchived"`
		CreatedAt       time.Time `json:"createdAt"`
		FullDatabaseId  *string   `json:"fullDatabaseId"` // A string? - crazy people outdoor nowadays
		Description     *string   `json:"description"`
		ForkCount       int       `json:"forkCount"`
		HtmlUrl         string    `json:"url"`
		PrimaryLanguage *struct {
			Name string `json:"name"`
		} `json:"primaryLanguage"`
		PushedAt   *time.Time `json:"pushedAt"`
		Stars      int        `json:"stargazerCount"`
		Visibility string     `json:"visibility"`
		Watchers   struct {
			TotalCount int `json:"totalCount"`
		} `json:"watchers"`
		Languages *struct {
			Nodes []struct {
				Name string `json:"name"`
			} `json:"nodes"`
		} `json:"languages"`
		RepositoryTopics struct {
			Nodes []struct {
				Topic struct {
					Name string `json:"name"`
				} `json:"topic"`
			} `json:"nodes"`
		} `json:"repositoryTopics"`
		LicenseInfo *struct {
			Name    string  `json:"name"`
			Body    string  `json:"body"`
			Key     string  `json:"key"`
			SpdxId  *string `json:"spdxId"`
			HtmlUrl *string `json:"url"`
			NodeId  string  `json:"id"`
		} `json:"licenseInfo"`
		PullRequest PullRequest `json:"pullRequests"`
		Issues      Issues      `json:"issues"`
		Releases    Releases    `json:"releases"`
		Object      *struct {
			Text *string `json:"text"`
		} `json:"object"`
	} `json:"nodes"`
}

// ==============================
// GRAPHQL FOR OAUTH2
// ==============================

type User struct {
	Login     string    `json:"login"`
	AvatarUrl string    `json:"avatarUrl"`
	Bio       *string   `json:"bio"`
	Company   *string   `json:"company"`
	CreatedAt time.Time `json:"createdAt"`
	Email     string    `json:"email"`
	Followers struct {
		TotalCount int `json:"totalCount"`
	} `json:"followers"`
	DatabaseID *int64 `json:"databaseId"`
	Status     *struct {
		Message *string `json:"message"`
	} `json:"status"`
	IsHireable bool       `json:"isHireable"`
	HtmlUrl    string     `json:"url"`
	Repository Repository `json:"repositories"`
}

type Data struct {
	User User `json:"user"`
}

// ==============================
// PUSH WEBHOOK ONLY (EVERYTHING BELOW)
// ==============================
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

type Repositor struct {
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
	HeadCommitID *HeadCommit `json:"head_commit,omitempty"`
	Compare      string      `json:"compare"`
	Forced       bool        `json:"forced"`
	Created      bool        `json:"created"`
	Deleted      bool        `json:"deleted"`
	CommitInfo   []Commit    `json:"commits"`
}
