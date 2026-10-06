package webhook

import (
	"encoding/json"
	"time"
)

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
	HeadCommitID *HeadCommit `json:"head_commit,omitempty"`
	Compare      string      `json:"compare"`
	Forced       bool        `json:"forced"`
	Created      bool        `json:"created"`
	Deleted      bool        `json:"deleted"`
	CommitInfo   []Commit    `json:"commits"`
}

// ==============================
// PULL REQUEST WEBHOOK ONLY (EVERYTHING BELOW) -- I HATE READING DOCUMENTATION
// ==============================
type Sender struct {
	ID    int64  `json:"sender"`
	Login string `json:"login"`
	Type  string `json:"type"`
}

type Author struct {
	ID    int64  `json:"id"`
	Login string `json:"string"`
	Type  string `json:"type"`
}

type PullRequestInfo struct {
	ID       int64   `json:"id"`
	Number   int     `json:"number"`
	Title    string  `json:"title"`
	State    string  `json:"open"`
	IsDraft  bool    `json:"is_draft"`
	IsMerged *bool   `json:"is_merged"`
	Auth     *Author `json:"author,omitempty"`
}

type TimeStamp struct {
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	ClosedAt  *time.Time `json:"closed_at"`
	MergedAt  *time.Time `json:"merged_at"`
}

type Metrics struct {
	Additions           int `json:"additions"`
	Deletions           int `json:"deletions"`
	ChangedFiles        int `json:"changed_files"`
	CommitsCount        int `json:"commits_count"`
	CommentsCount       int `json:"comments_count"`
	ReviewCommentsCount int `json:"review_comments_count"`
}

type Git struct {
	HeadBranch     string  `json:"head_branch"`
	HeadSha        string  `json:"head_sha"`
	BaseBranch     string  `json:"base_branch"`
	MergeCommitSha *string `json:"merge_commit_sha"`
}

type Workflow struct {
	AssigneeIDs          []int64  `json:"assignee_ids"`
	RequestedReviewerIDs []int64  `json:"requested_reviewer_ids"`
	RequestedTeamIDs     []int64  `json:"requested_team_ids"`
	Labels               []string `json:"labels"`
	MilestoneID          *int64   `json:"milestone_id"`
}

type GitHubPullRequest struct {
	EventAction string          `json:"event"`
	RepoInfo    Repository      `json:"repository"`
	SentBy      Sender          `json:"sender"`
	PRInfo      PullRequestInfo `json:"pull_request"`
	TimeStamp   TimeStamp       `json:"timestamps"`
	Metrics     Metrics         `json:"metrics"`
	GitInfo     Git             `json:"git"`
	Workflow    Workflow        `json:"workflow"`
}

// ==============================
// ISSUES WEBHOOK ONLY (EVERYTHING BELOW)
// ==============================

type Assignee struct {
	ID   int64  `json:"id"`
	Name string `json:"login"`
	Type string `json:"type"`
}

type Label struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
}

type Milestone struct {
	Description *string `json:"description"`
	DueOn       *string `json:"due_on"`
	State       string  `json:"state"`
	Title       string  `json:"title"`
}

type Reactions struct {
	TotalCount       int `json:"total_count"`
	PositiveReaction int `json:"+1"`
	NegativeReaction int `json:"-1"`
	Confused         int `json:"confused"`
	Eyes             int `json:"eyes"`
	Heart            int `json:"heart"`
	Hooray           int `json:"hooray"`
	Laugh            int `json:"laugh"`
	Rocket           int `json:"rocket"`
}

type SubIssuesSummary struct {
	Total            int `json:"total"`
	Completed        int `json:"completed"`
	PercentCompleted int `json:"percent_completed"`
}

type IssueDependenciesSummary struct {
	TotalBlockedBy int `json:"total_blocked_by"`
	TotalBlocking  int `json:"total_blocking"`
}

type SingleSelectOption struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type MultiSelectOption struct {
	MultiSelectOptionID int64  `json:"id"`
	Name                string `json:"name"`
}

type IssueFieldValue struct {
	IssueFieldName     string               `json:"issue_field_name"`
	DataType           string               `json:"data_type"`
	Value              json.RawMessage      `json:"value"`
	SingleSelectOption *SingleSelectOption  `json:"single_select_option,omitempty"`
	MultiSelectOptions *[]MultiSelectOption `json:"multi_select_options,omitempty"`
}

type Type struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
}

type User struct {
	ID   int64  `json:"id"`
	Name string `json:"login"`
	Type string `json:"type"`
}

type Issue struct {
	ID              int64      `json:"id"`
	AuthAssociation string     `json:"author_association"`
	Body            *string    `json:"body"`
	Comments        int        `json:"comments"`
	Draft           bool       `json:"draft"`
	CreatedAt       time.Time  `json:"created_at"`
	ClosedAt        *time.Time `json:"closed_at"`
	Labels          []Label    `json:"labels"`
	Locked          bool       `json:"locked"`
	Milestone       *Milestone `json:"milestone,omitempty"`
	Number          int        `json:"number"`
	Reactions       Reactions  `json:"reactions"`
	// Only open/closed
	State                    string                   `json:"state"`
	StateReason              *string                  `json:"state_reason"`
	SubIssuesSummary         SubIssuesSummary         `json:"sub_issues_summary"`
	IssueDependenciesSummary IssueDependenciesSummary `json:"issue_dependencies_summary"`
	IssueFieldValues         []IssueFieldValue        `json:"issue_field_values"`
	Title                    string                   `json:"title"`
	Type                     *Type                    `json:"type,omitempty"`
	UpdatedAt                time.Time                `json:"updated_at"`
	User                     *User                    `json:"user,omitempty"`
	Assignee                 *Assignee                `json:"assignee"`
}

type GitHubIssues struct {
	RepoInfo     Repository `json:"repository"`
	Action       string     `json:"action"`
	AssigneeInfo *Assignee  `json:"assignee,omitempty"`
	IssueInfo    Issue      `json:"issue"`
	Sender       Sender     `json:"sender"`
}

// ==============================
// ISSUE COMMENT WEBHOOK ONLY (EVERYTHING BELOW)
// ==============================

type Body struct {
	From string `json:"from"`
}
type Change struct {
	Body Body `json:"body"`
}

type Comment struct {
	ID                int64     `json:"id"`
	User              User      `json:"user"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	AuthorAssociation string    `json:"author_association"`
	Body              string    `json:"body"`
	Reactions         Reactions `json:"reactions"`
}

type GitHubIssueComment struct {
	Action     string     `json:"action"`
	Changes    *Change    `json:"changes,omitempty"`
	Comment    Comment    `json:"comment"`
	Issue      Issue      `json:"issue"`
	Repository Repository `json:"repository"`
	Sender     Sender     `json:"sender"`
}

// ==============================
// RELEASES WEBHOOK ONLY (EVERYTHING BELOW)
// =============================

type Release struct {
	ID              int64      `json:"id"`
	TagName         string     `json:"tag_name"`
	Name            string     `json:"name"`
	Body            string     `json:"body"`
	TargetCommitish string     `json:"target_commitish"`
	Draft           bool       `json:"draft"`
	Prerelease      bool       `json:"prerelease"`
	PublishedAt     *time.Time `json:"published_at"`
	Author          Author     `json:"author"`
}

type GitHubReleases struct {
	Action     string     `json:"action"`
	Release    Release    `json:"release"`
	Repository Repository `json:"repository"`
	Sender     Sender     `json:"sender"`
}
