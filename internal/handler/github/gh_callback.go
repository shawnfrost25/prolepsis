package oauthGithub

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"prolepsis/internal/auth"
	db "prolepsis/internal/db/sqlc"
	"prolepsis/internal/lib"
	"strconv"
	"time"

	"github.com/bytedance/sonic"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

type QueryLookup struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables"`
}

type GitHubUser struct {
	GitHubUsername     string    `json:"login"`
	GitHubID           int64     `json:"id"`
	AvatarURL          string    `json:"avatar_url"`
	GitHubHTMLURL      string    `json:"html_url"`
	GitHubCompany      *string   `json:"company"`
	GitHubEmail        *string   `json:"email"`
	GitHubHireable     *bool     `json:"hireable"`
	GitHubBio          *string   `json:"bio"`
	GitHubFollowers    *int32    `json:"followers"`
	GitHubPublicRepos  int16     `json:"public_repos"`
	GitHubPrivateRepos int16     `json:"total_private_repos"`
	GitHubCreatedAt    time.Time `json:"created_at"`
}

type GitHubRepoLicense struct {
	Key    string  `json:"key"`
	Name   string  `json:"name"`
	SpdxID string  `json:"spdx_id"`
	URL    *string `json:"url"`
	NodeID string  `json:"node_id"`
}

type GitHubRepo struct {
	RepoName        string             `json:"name"`
	RepoID          int64              `json:"id"`
	RepoHTMLURL     string             `json:"html_url"`
	RepoDescription *string            `json:"description"`
	RepoCreatedAt   time.Time          `json:"created_at"`
	RepoPushedAt    time.Time          `json:"pushed_at"`
	RepoStars       *int32             `json:"stargazers_count"`
	RepoWatches     *int32             `json:"watchers_count"`
	RepoForked      bool               `json:"fork"`
	RepoForksCount  *int32             `json:"forks_count"`
	RepoLanguage    *string            `json:"language"`
	RepoLicense     *GitHubRepoLicense `json:"license"`
	RepoTopics      []string           `json:"topics"`
	RepoArchived    bool               `json:"archived"`
	RepoVisibility  string             `json:"visibility"`
}

type GitHubWebhookConfig struct {
	URL         string `json:"url"`
	ContentType string `json:"content_type"`
	Secret      string `json:"secret"`
	InsecureSSL int    `json:"insecure_ssl"`
}
type GitHubWebhookCreation struct {
	Name   string              `json:"name"`
	Active bool                `json:"active"`
	Events []string            `json:"events"`
	Config GitHubWebhookConfig `json:"config"`
}

type WebhookResponse struct {
	ID int64 `json:"id"`
}

func (h *GitHubHandler) GitHubCallback(w http.ResponseWriter, r *http.Request) {
	logger := zerolog.Ctx(r.Context()).With().Str("handler", "GitHubCallback").Logger()
	u, ok := auth.FetchContextInsideHandler(w, r)
	if !ok {
		return
	}
	timeout, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	// Format:
	// Queries -> Explain what you can query through
	// -- user -> Means you can query through "user"
	// -- -- login (String!) -> Means that it must have a parameter called "login" - whose value must be a string (not null)
	// -- viewer -> Means you can query through "viewer"
	// -- -- <> -> Means that there is no parameter that you need to give it
	// Objects -> Explain what fields the two queries can accept

	query := `
		query GetGithubInfo($login: String!) {
			user(login: $login) {
				login
				avatarUrl
				bio # CAN BE NULL
				company # CAN BE NULL
				createdAt
				email
				followers {
					totalCount
				}
				databaseId # CAN BE NULL (WHY?!)
				status { # CAN BE NULL
					message # CAN BE NULL
				}
				isHireable
				url
				repositories(affiliations: [OWNER], isFork: false, first: 100) { # A LIST
					totalCount 
					nodes { # A LIST // CAN BE NULL
						name
						isArchived
						createdAt
						fullDatabaseId # CAN BE NULL
						description # CAN BE NULL
						forkCount
						url
						primaryLanguage  { # CAN BE NULL
							name
						}
						pushedAt # CAN BE NULL
						stargazerCount
						visibility
						watchers {
							totalCount
						}
						languages(first: 5) { # CAN BE NULL
							nodes { # A LIST // CAN BE NULL
								name 
							}
						}
						repositoryTopics(first: 40) {
							nodes { # A LIST // CAN BE NULL
								topic {
									name
								}
							}
						}
						licenseInfo { # CAN BE NULL
							name
							body
							key
							spdxId # CAN BE NULL
							url # CAN BE NULL
							id
						}
						pullRequests(first: 15, states: [OPEN, CLOSED, MERGED], orderBy: { field: CREATED_AT, direction: DESC }) {
							nodes { # A LIST // CAN BE NULL
								additions
								author { # CAN BE NULL
									login
								}
								body
								changedFiles
								comments(first: 10, orderBy: { field: CREATED_AT, direction: DESC }) {
									nodes { # A LIST // CAN BE NULL
										author { # CAN BE NULL
											login
										}
										body
										fullDatabaseId # CAN BE NULL
										isMinimized
										minimizedReason # CAN BE NULL
										createdAt
										updatedAt
										lastEditedAt # CAN BE NULL
										url
									}
								}
								fullDatabaseId
								deletions
								isDraft
								locked
								merged
								closed
								number
								reactionGroups { # A LIST
									content # The emoji
									reactors { # The actual count of the emoji
										totalCount
									}
								}
								participants {
									totalCount
								}
								labels(first: 5) {
									nodes { # A LIST // CAN BE NULL
										name
									}
								}
								state
								url
								title
								createdAt
								updatedAt
								closedAt
								mergedAt
							}
						}
						issues(first: 20, states: [OPEN, CLOSED], orderBy: { field: CREATED_AT, direction: DESC }) {
							nodes { # A LIST // CAN BE NULL
								author { # CAN BE NULL
									login
								}
								body
								comments(first: 10, orderBy: { field: CREATED_AT, direction: DESC }) {
									nodes { # A LIST // CAN BE NULL
										author { # CAN BE NULL
											login
										}
										body
										fullDatabaseId # CAN BE NULL
										isMinimized
										minimizedReason # CAN BE NULL
										createdAt
										updatedAt
										lastEditedAt # CAN BE NULL
										url
									}
								}
								closed
								createdAt
								closedAt
								fullDatabaseId
								labels(first: 5) {
									nodes { # A LIST // CAN BE NULL
										name
									}
								}
								issueType { # CAN BE NULL
									name
								}
								lastEditedAt
								locked
								number
								reactionGroups { # A LIST
									content # The emoji
									reactors { # The actual count of the emoji
										totalCount
									}
								}
								participants {
									totalCount
								}
								state
								stateReason # CAN BE NULL
								title
								url
							}
						}
						releases(first: 3, orderBy: { field: CREATED_AT, direction: DESC }) {
							totalCount
							nodes {	# A LIST // CAN BE NULL
								databaseId # CAN BE NULL
								author { # CAN BE NULL
									login
								}
								description # CAN BE NULL
								immutable
								isPrerelease
								isDraft
								url
								tagName
								createdAt
								updatedAt
								publishedAt
							}
						}
						object(expression: "HEAD:README.md") { # CAN BE NULL
							... on Blob {
								text # CAN BE NULL
							}
						}
					}
				}
			}
		}
	`

	stateURL := r.URL.Query().Get("state")
	if stateURL == "" {
		logger.Error().
			Int("status", http.StatusUnauthorized).
			Str("cause", "missing_state_parameter").
			Msg("missing state query parameter")

		lib.Pretty(w, http.StatusUnauthorized, lib.Error{
			Code:    "UNAUTHORIZED",
			Message: "Missing state query parameter",
			Details: map[string]string{
				"if_self":        "the state is missing, please, retry the OAuth2 flow",
				"if_third_party": "do not click links sent by individuals (or 'friends'), as they may be attempts to perform CSRF attacks",
			},
			TraceID: u.Trace,
		})

		return
	}

	id, err := h.RedisClient.GetDel(timeout, "oauth:github:state:"+stateURL).Result()
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			logger.Error().
				Err(err).
				Int("status", http.StatusRequestTimeout).
				Str("cause", "timeout").
				Msg("couldn't match the given state due to timeout")
			lib.Pretty(w, http.StatusRequestTimeout, lib.Error{
				Code:    "REQUEST_TIMEOUT",
				Message: "Timeout while trying to match the given state",
				TraceID: u.Trace,
			})
			return
		}
		if errors.Is(err, redis.Nil) {
			logger.Warn().
				Err(err).
				Int("status", http.StatusUnauthorized).
				Str("cause", "expired_or_non_existent_state").
				Msg("the provided state is not-existent or expired")
			lib.Pretty(w, http.StatusUnauthorized, lib.Error{
				Code:    "UNAUTHORIZED",
				Message: "The provided state is expired",
				TraceID: u.Trace,
			})
			return
		}
		logger.Error().
			Int("status", http.StatusInternalServerError).
			Str("cause", "unrecognized").
			Msg("unrecognized error while trying to match the given state")
		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "Unrecognized error while matching the given state",
			TraceID: u.Trace,
		})
		return
	}

	if id != u.ID.String() {
		logger.Warn().
			Int("status", http.StatusUnauthorized).
			Str("cause", "state_user_mismatch").
			Msg("state belongs to a different user")
		lib.Pretty(w, http.StatusUnauthorized, lib.Error{
			Code:    "UNAUTHORIZED",
			Message: "The OAuth state does not match the current user",
			TraceID: u.Trace,
		})
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		logger.Warn().
			Int("status", http.StatusUnauthorized).
			Str("cause", "missing_code_parameter").
			Msg("the given redirection URL doesn't contain a code")
		lib.Pretty(w, http.StatusUnauthorized, lib.Error{
			Code:    "UNAUTHORIZED",
			Message: "Missing code for token exchange",
			TraceID: u.Trace,
		})
		return
	}

	token, err := h.GH_Config.Exchange(timeout, code)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			logger.Error().
				Err(err).
				Int("status", http.StatusRequestTimeout).
				Str("cause", "timeout").
				Msg("couldn't perform the code-to-token exchnage due to timeout")
			lib.Pretty(w, http.StatusRequestTimeout, lib.Error{
				Code:    "REQUEST_TIMEOUT",
				Message: "Timeout while exchanging",
				TraceID: u.Trace,
			})
			return
		}
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("cause", "unrecognized").
			Msg("unrecognized error while trying to perform the code-to-token")
		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "Unrecognized error while exchanging",
			TraceID: u.Trace,
		})
		return
	}

	////////////////////////////////

	ERRORRRRRRRRRRRRRRRRRRRRRRRRRRRRRRRRRRRRRRRRRRRRRRRRRRRRRRRRRRRRRRRRRRR

	////////////////////////////////

	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("cause", "failed_rand_encryption").
			Msg("failed to encrypt (via rand) the given bytes due to an unrecognized error")
		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "Failed rand encryption",
			TraceID: u.Trace,
		})
		return
	}
	secret := hex.EncodeToString(secretBytes)

	webhookCreate := GitHubWebhookCreation{
		Name:   "web",
		Active: true,
		Events: []string{"push", "pull_request", "issues", "issue_comment", "release", "repository"},
		Config: GitHubWebhookConfig{
			URL:         "",
			ContentType: "json",
			Secret:      secret,
			InsecureSSL: 0,
		},
	}

	bodyBytes, err := sonic.Marshal(webhookCreate)
	if err != nil {
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("cause", "failed_body_conversion").
			Msg("failed to turn the given structure into a body for github webhook creation")
		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "Failed to convert structure for webhook creation",
			TraceID: u.Trace,
		})
		return
	}
	body := bytes.NewReader(bodyBytes)

	webhookURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/hooks", reqUser.GitHubUsername, reqRepo.RepoName)
	reqW, err := http.NewRequestWithContext(timeout, http.MethodPost, webhookURL, body)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			logger.Error().
				Err(err).
				Int("status", http.StatusRequestTimeout).
				Str("cause", "timeout").
				Int16("repo_number", repo).
				Str("repo_name", reqRepo.RepoName).
				Msg("couldn't create webhook creation request due to timeout")
			lib.Pretty(w, http.StatusRequestTimeout, lib.Error{
				Code:    "REQUEST_TIMEOUT",
				Message: "Timedout while trying to create the request for the webhook creation",
				TraceID: u.Trace,
			})
			return
		}
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("cause", "unrecognized").
			Int16("repo_number", repo).
			Str("repo_name", reqRepo.RepoName).
			Msg("unrecognized error while trying to create a request for the webhook creation")
		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "Failed to create a webhook",
			TraceID: u.Trace,
		})
		return
	}
	reqW.Header.Set("Accept", "application/vnd.github.json")
	reqW.Header.Set("Authorization", "Bearer "+token.AccessToken)
	reqW.Header.Set("X-GitHub-Api-Version", "2026-03-10")

	repoIDStr := strconv.FormatInt(reqRepo.RepoID, 10)

	_, err = h.RedisClient.Set(timeout, "oauth:github:webhook:secret:"+repoIDStr, secret, 0).Result()
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			logger.Error().
				Err(err).
				Int("status", http.StatusRequestTimeout).
				Str("cause", "timeout").
				Msg("failed to insert the github webhook secret inside redis due to timeout")
			lib.Pretty(w, http.StatusRequestTimeout, lib.Error{
				Code:    "REQUEST_TIMEOUT",
				Message: "Timedout while trying to insert the webhook secret",
				TraceID: u.Trace,
			})
			return
		}
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("cause", "unrecognized").
			Msg("unrecognized error while trying to insert the github webhook secret inside redis")
		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "Hit an unrecognized error while trying to insert the webhook secret",
			TraceID: u.Trace,
		})
		return
	}

	wasRemoved, err := h.RedisClient.Persist(timeout, "oauth:github:webhook:secret:"+repoIDStr).Result()
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			logger.Error().
				Err(err).
				Int("status", http.StatusRequestTimeout).
				Str("cause", "timeout").
				Msg("failed to make the redis webhook secret persistent due to timeout")
			lib.Pretty(w, http.StatusRequestTimeout, lib.Error{
				Code:    "REQUEST_TIMEOUT",
				Message: "Timedout while trying to make the secret persistent",
				TraceID: u.Trace,
			})
			return
		}
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("cause", "unrecognized").
			Msg("hit an unrecognized error while trying to make the redis webhook secret persistent")
		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "Unrecognized error while trying to make the secret persistent",
			TraceID: u.Trace,
		})
		return
	}
	if !wasRemoved {
		logger.Error().
			Int("status", http.StatusInternalServerError).
			Str("cause", "failed_ttl_removal").
			Msg("failed to remove the redis webhook secret TTL to make the key persistent")
		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "Failed secret TTL removal",
			TraceID: u.Trace,
		})
		return
	}

	respWeb, err := h.HTTP_Client.Do(reqW)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			logger.Error().
				Err(err).
				Int("status", http.StatusRequestTimeout).
				Str("cause", "timeout").
				Msg("failed to send request and get response about webhook creation due to timeout")
			lib.Pretty(w, http.StatusRequestTimeout, lib.Error{
				Code:    "REQUEST_TIMEOUT",
				Message: "Timedout while trying to send request about webhook creation",
				TraceID: u.Trace,
			})
			return
		}
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("cause", "unrecognized").
			Msg("unrecognized error while trying to send request to get response about webhook creation")
		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "Unrecognized error while trying to send the request about webhook creation",
			TraceID: u.Trace,
		})
		return
	}

	var reqWeb WebhookResponse
	err = sonic.ConfigDefault.NewDecoder(respWeb.Body).Decode(&reqWeb)
	if err != nil {
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("cause", "failed_to_decode").
			Msg("failed to decode the webhook creation response")
		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "Failed to decode the webhook creation response",
			TraceID: u.Trace,
		})
		return
	}

	does_not_exists, err := h.RedisClient.SetNX(timeout, "oauth:github:webhook:hook_id:"+repoIDStr, reqWeb.ID, 0).Result()
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			logger.Error().
				Err(err).
				Int("status", http.StatusRequestTimeout).
				Str("cause", "timeout").
				Msg("failed to insert the webhook id inside redis due to timeout")
			lib.Pretty(w, http.StatusRequestTimeout, lib.Error{
				Code:    "REQUEST_TIMEOUT",
				Message: "Timedout while trying to insert the webhook id inside redis",
				TraceID: u.Trace,
			})
			return
		}
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("cause", "unrecognized").
			Msg("failed to insert the webhook id inside redis due to unrecognized error")
		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "Unrecognized error while trying to insert the webhook id inside redis",
			TraceID: u.Trace,
		})
		return
	}
	if !does_not_exists {
		logger.Debug().
			Int("status", http.StatusConflict).
			Int64("repo_id", reqRepo.RepoID).
			Str("repo_name", reqRepo.RepoName).
			Msg("webhook id already exists, everything is healthy, skipping")
	}

	wasRemoved, err = h.RedisClient.Persist(timeout, "oauth:github:webhook:hook_id:"+repoIDStr).Result()
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			logger.Error().
				Err(err).
				Int("status", http.StatusRequestTimeout).
				Str("cause", "timeout").
				Msg("failed to make the redis webhook id persistent due to timeout")
			lib.Pretty(w, http.StatusRequestTimeout, lib.Error{
				Code:    "REQUEST_TIMEOUT",
				Message: "Timedout while trying to make the webhook id persistent",
				TraceID: u.Trace,
			})
			return
		}
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("cause", "unrecognized").
			Msg("hit an unrecognized error while trying to make the redis webhook id persistent")
		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "Unrecognized error while trying to make the webhook id persistent",
			TraceID: u.Trace,
		})
		return
	}
	if !wasRemoved {
		logger.Error().
			Int("status", http.StatusInternalServerError).
			Str("cause", "failed_ttl_removal").
			Msg("failed to remove the redis webhook id TTL to make the key persistent")
		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "Failed webhook id TTL removal",
			TraceID: u.Trace,
		})
		return
	}

	logger.Info().
		Int("status", http.StatusOK).
		Str("cause", "successfully_finished_github_oauth").
		Msg("managed to successfully insert GitHub data inside the database and create webhook")
	lib.Pretty(w, http.StatusOK, lib.Error{
		Code:    "OK",
		Message: "successfully inserted github data to database",
		TraceID: u.Trace,
	})
}

// Nearly 950 Lines of this mess, GOD, THIS STUFF IS IMPOSSIBLE TO DEBUG, UGHHHHHHHHHHHHHHh
