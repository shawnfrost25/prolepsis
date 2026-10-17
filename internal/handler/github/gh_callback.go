package oauthGithub

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"prolepsis/internal/auth"
	db "prolepsis/internal/db/sqlc"
	"prolepsis/internal/errlog"
	synchelper "prolepsis/internal/handler/github/sync_helper"
	"prolepsis/internal/lib"
	"strconv"
	"sync"
	"time"

	"github.com/bytedance/sonic"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

type QueryLookup struct {
	Query string `json:"query"`
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
	logger := zerolog.Ctx(r.Context()).With().Str("op", "github_callback").Logger()
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
		query GetGithubInfo {
			viewer {
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
							url # CAN BE NULL (IF CUSTOM)
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
										url
									}
								}
								fullDatabaseId # CAN BE NULL
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
								closedAt # CAN BE NULL
								mergedAt # CAN BE NULL
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
										url
									}
								}
								closed
								createdAt
								closedAt # CAN BE NULL
								updatedAt
								fullDatabaseId # CAN BE NULL
								labels(first: 5) {
									nodes { # A LIST // CAN BE NULL
										name
									}
								}
								issueType { # CAN BE NULL
									name
								}
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
								publishedAt # CAN BE NULL
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
		errlog.EmptyValueError(logger, "oauth_state", w, u.Trace)
		return
	}

	id, err := h.RedisClient.GetDel(timeout, "oauth:github:state:"+stateURL).Result()
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			errlog.DeadlineExceededError(logger, "state_fetch", w, u.Trace, err)
			return
		}
		if redis.IsAuthError(err) {
			errlog.RedisAuthenticationError(logger, w, u.Trace, err)
			return
		}
		if errors.Is(err, redis.Nil) {
			errlog.NotFoundError(logger, "oauth_state", w, u.Trace, err)
			return
		}
		errlog.UnexpectedError(logger, "state_fetch", w, u.Trace, err)
		return
	}

	if id != u.ID.String() {
		errlog.UnauthorizedError(logger, "state_check", w, u.Trace, nil)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		errlog.EmptyValueError(logger, "oauth_code", w, u.Trace)
		return
	}

	token, err := h.GH_Config.Exchange(timeout, code)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			errlog.DeadlineExceededError(logger, "code_exchange", w, u.Trace, err)
			return
		}
		errlog.UnexpectedError(logger, "code_exchange", w, u.Trace, err)
		return
	}

	bodyGraphQlRaw := QueryLookup{
		Query: query,
	}

	bodyGraphQl, err := sonic.Marshal(bodyGraphQlRaw)
	if err != nil {
		errlog.ParsingError(logger, "graphql_marshal_format", w, u.Trace, err)
		return
	}

	reqGraphQl, err := http.NewRequestWithContext(timeout, http.MethodPost, "https://api.github.com/graphql", bytes.NewReader(bodyGraphQl))
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			errlog.DeadlineExceededError(logger, "graphql_request", w, u.Trace, err)
			return
		}
		errlog.UnexpectedError(logger, "graphql_request", w, u.Trace, err)
		return
	}
	reqGraphQl.Header.Set("Authorization", "Bearer "+token.AccessToken)
	reqGraphQl.Header.Set("Content-Type", "application/json")
	reqGraphQl.Header.Set("User-Agent", "Prolepsis-IWTLYTYDD")

	respGraphQl, err := h.HTTP_Client.Do(reqGraphQl)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			errlog.DeadlineExceededError(logger, "graphql_response", w, u.Trace, err)
			return
		}
		errlog.UnexpectedError(logger, "graphql_response", w, u.Trace, err)
		return
	}
	defer func() {
		err := respGraphQl.Body.Close()
		if err != nil {
			errlog.UnexpectedError(logger, "graphql_response_close", w, u.Trace, err)
			return
		}
	}()

	if respGraphQl.StatusCode != 200 {
		logger = logger.With().Int("response_code", respGraphQl.StatusCode).Logger()
		errlog.UnexpectedError(logger, "graphql_response", w, u.Trace, err)
		return
	}

	var synErr *json.SyntaxError
	var typeErr *json.UnmarshalTypeError

	var graph *synchelper.User
	err = sonic.ConfigDefault.NewDecoder(respGraphQl.Body).Decode(&graph)
	if err != nil {
		if errors.As(err, &synErr) {
			errlog.JsonSyntaxError(logger, "graph_decode_payload", w, u.Trace, synErr)
			return
		}
		if errors.As(err, &typeErr) {
			errlog.JsonUnmarshalTypeError(logger, w, u.Trace, typeErr)
			return
		}
		errlog.ParsingError(logger, "graph_decode_payload", w, u.Trace, err)
		return
	}

	if graph.DatabaseID == nil {
		errlog.EmptyValueError(logger, "github_id", w, u.Trace)
		return
	}

	var email *string
	if graph.Email != "" {
		email = &graph.Email
	}

	var pgErr *pgconn.PgError

	err = h.Queries.InsertGitHubUser(timeout, db.InsertGitHubUserParams{
		UserID:        u.ID,
		GithubID:      *graph.DatabaseID,
		Login:         graph.Login,
		Company:       graph.Company,
		Email:         email,
		Bio:           graph.Bio,
		StatusMessage: graph.Status.Message,
		Followers:     int32(graph.Followers.TotalCount),
		TotalRepos:    int32(graph.Repository.TotalCount),
		IsHireable:    graph.IsHireable,
		AvatarUrl:     graph.AvatarUrl,
		HtmlUrl:       graph.HtmlUrl,
		CreatedAt:     pgtype.Timestamptz{Time: graph.CreatedAt, Valid: true},
	})
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			errlog.DeadlineExceededError(logger, "user_insert", w, u.Trace, err)
			return
		}
		if errors.Is(err, pgconn.ErrConnClosed) {
			errlog.ErrConnClosed(logger, w, u.Trace, err)
			return
		}
		if errors.As(err, &pgErr) {
			errlog.PgConnError(logger, w, u.Trace, "user_insert_failed", pgErr, err)
		}
		errlog.UnexpectedError(logger, "user_insert", w, u.Trace, err)
		return
	}

	var wg sync.WaitGroup

	parseStr := func(fullDatabaseId *string) (int64, error, string) {
		if fullDatabaseId == nil || *fullDatabaseId == "" {
			return 0, errors.New("Error: Required value 'fullDatabaseId' cannot be empty"), "_id_invalid"
		}
		id, err := strconv.ParseInt(*fullDatabaseId, 10, 64)
		if err != nil {
			return 0, err, "_id_malformed"
		}
		if id <= 0 {
			return 0, errors.New("Error: Value 'fullDatabaseId' cannot be 0 or negative"), "_id_format_invalid"
		}
		return id, nil, ""
	}

	if len(graph.Repository.Nodes) != 0 {
		for _, repo := range graph.Repository.Nodes {
			repositoryId, err, code := parseStr(repo.FullDatabaseId)
			if err != nil {
				logger = logger.With().Str("repo_name", repo.Name).Str("repo_id", *repo.FullDatabaseId).Logger()
				logger.Debug().
					Err(err).
					Int("status", http.StatusBadRequest).
					Str("code", "repo"+code).
					Msg("repository id did not meet requirements")
				continue
			}

			var licenseKey *string
			if repo.LicenseInfo != nil {
				licenseKey = &repo.LicenseInfo.Key
				err := h.Queries.InsertGitHubLicense(timeout, db.InsertGitHubLicenseParams{
					SpdxID:     repo.LicenseInfo.SpdxId,
					NodeID:     repo.LicenseInfo.NodeId,
					LicenseKey: repo.LicenseInfo.Key,
					Name:       repo.LicenseInfo.Name,
					Body:       repo.LicenseInfo.Body,
					HtmlUrl:    repo.LicenseInfo.HtmlUrl,
				})
				if err != nil {
					logger.Debug().
						Err(err).
						Int("status", http.StatusInternalServerError).
						Str("code", "license_insert_failed").
						Str("license_name", repo.LicenseInfo.Name).
						Str("license_key", repo.LicenseInfo.Key).
						Msg("license insertion failed")
					continue
				}
			}

			var languages []string
			if repo.Languages != nil {
				if len(repo.Languages.Nodes) != 0 {
					for _, language := range repo.Languages.Nodes {
						if language.Name == "" {
							continue
						}
						languages = append(languages, language.Name)
					}
				}
			}

			var topics []string
			if len(repo.RepositoryTopics.Nodes) != 0 {
				for _, topic := range repo.RepositoryTopics.Nodes {
					if topic.Topic.Name == "" {
						continue
					}
					topics = append(topics, topic.Topic.Name)
				}
			}

			var readme *string
			if repo.Object != nil && repo.Object.Text != nil {
				readme = repo.Object.Text
			}

			var mainLanguage *string
			if repo.PrimaryLanguage != nil {
				mainLanguage = &repo.PrimaryLanguage.Name
			}

			var pushedAt pgtype.Timestamptz
			if repo.PushedAt != nil {
				pushedAt = pgtype.Timestamptz{Time: *repo.PushedAt, Valid: true}
			}

			err = h.Queries.InsertGitHubRepo(timeout, db.InsertGitHubRepoParams{
				GithubID:           *graph.DatabaseID,
				RepoID:             repositoryId,
				Name:               repo.Name,
				Description:        repo.Description,
				Readme:             readme,
				MainLanguage:       mainLanguage,
				SecondaryLanguages: languages,
				LicenseKey:         licenseKey,
				Topics:             topics,
				Visibility:         repo.Visibility,
				Stars:              int32(repo.Stars),
				Watches:            int32(repo.Watchers.TotalCount),
				ForksCount:         int32(repo.ForkCount),
				IsArchived:         repo.IsArchived,
				HtmlUrl:            repo.HtmlUrl,
				CreatedAt:          pgtype.Timestamptz{Time: repo.CreatedAt, Valid: true},
				PushedAt:           pushedAt,
			})
			if err != nil {
				logger.Debug().
					Err(err).
					Int("status", http.StatusInternalServerError).
					Str("code", "repo_insert_failed").
					Str("repo_name", repo.Name).
					Int64("repo_id", repositoryId).
					Msg("repository insertion failed")
				continue
			}

			if len(repo.PullRequest.Nodes) != 0 {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for _, pullRequest := range repo.PullRequest.Nodes {
						failed := synchelper.PullRequestCallback(logger, pullRequest, timeout, repositoryId, *graph.DatabaseID, h.Queries)
						if failed {
							continue
						}
					}
				}()
			}
			if len(repo.Issues.Nodes) != 0 {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for _, issue := range repo.Issues.Nodes {
						failed := synchelper.IssueCallback(logger, issue, timeout, repositoryId, *graph.DatabaseID, h.Queries)
						if failed {
							continue
						}
					}
				}()
			}
			if len(repo.Releases.Nodes) != 0 {
				wg.Add(1)
				go func() {
					wg.Done()
					for _, release := range repo.Releases.Nodes {
						failed := synchelper.ReleaseCallback(logger, release, timeout, repositoryId, *graph.DatabaseID, h.Queries)
						if failed {
							continue
						}
					}
				}()
			}
			wg.Wait()

			logger = logger.With().
				Int64("repo_id", repositoryId).
				Str("repo_name", repo.Name).
				Logger()

			secretBytes := make([]byte, 32)
			if _, err := rand.Read(secretBytes); err != nil {
				logger.Debug().
					Err(err).
					Int("status", http.StatusInternalServerError).
					Str("code", "webhook_secret_generate_failed").
					Msg("failed to generate webhook secret")
				continue
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
				logger.Debug().
					Err(err).
					Int("status", http.StatusBadRequest).
					Str("code", "webhook_request_marshal_failed").
					Msg("failed to marshal webhook request")
				continue
			}

			body := bytes.NewReader(bodyBytes)
			webhookURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/hooks", graph.Login, repo.Name)

			reqW, err := http.NewRequestWithContext(timeout, http.MethodPost, webhookURL, body)
			if err != nil {
				logger.Debug().
					Err(err).
					Int("status", http.StatusInternalServerError).
					Str("code", "webhook_request_create_failed").
					Msg("failed to create webhook request")
				continue
			}

			reqW.Header.Set("Accept", "application/vnd.github.json")
			reqW.Header.Set("Authorization", "Bearer "+token.AccessToken)
			reqW.Header.Set("X-GitHub-Api-Version", "2026-03-10")

			repoIDStr := strconv.FormatInt(repositoryId, 10)

			_, err = h.RedisClient.Set(timeout, "oauth:github:webhook:secret:"+repoIDStr, secret, 0).Result()
			if err != nil {
				logger.Debug().
					Err(err).
					Int("status", http.StatusInternalServerError).
					Str("code", "webhook_secret_insert_failed").
					Msg("failed to insert webhook secret")
				continue
			}

			wasRemoved, err := h.RedisClient.Persist(timeout, "oauth:github:webhook:secret:"+repoIDStr).Result()
			if err != nil {
				logger.Debug().
					Err(err).
					Int("status", http.StatusInternalServerError).
					Str("code", "webhook_secret_ttl_remove_failed").
					Msg("failed to remove webhook secret TTL")
				continue
			}

			if !wasRemoved {
				logger.Debug().
					Int("status", http.StatusInternalServerError).
					Str("code", "webhook_secret_ttl_remove_failed").
					Msg("failed to remove webhook secret TTL")
				continue
			}

			respWeb, err := h.HTTP_Client.Do(reqW)
			if err != nil {
				errlog.UnexpectedError(logger, "webhook_request", w, u.Trace, err)
				return
			}

			var reqWeb WebhookResponse
			err = sonic.ConfigDefault.NewDecoder(respWeb.Body).Decode(&reqWeb)
			if err != nil {
				if errors.As(err, &synErr) {
					errlog.JsonSyntaxError(logger, "webhook_request_payload", w, u.Trace, synErr)
					return
				}
				if errors.As(err, &typeErr) {
					errlog.JsonUnmarshalTypeError(logger, w, u.Trace, typeErr)
					return
				}
				errlog.ParsingError(logger, "webhook_request_payload", w, u.Trace, err)
				return
			}

			doesNotExist, err := h.RedisClient.SetNX(
				timeout,
				"oauth:github:webhook:hook_id:"+repoIDStr,
				reqWeb.ID,
				0,
			).Result()

			if err != nil {
				logger.Error().
					Err(err).
					Int("status", http.StatusInternalServerError).
					Str("code", "webhook_id_insert_failed").
					Msg("failed to insert webhook ID")

				lib.Pretty(w, http.StatusInternalServerError, lib.Error{
					Code:    "INTERNAL_SERVER_ERROR",
					Message: "Failed to insert webhook ID",
					TraceID: u.Trace,
				})
				return
			}

			if !doesNotExist {
				logger.Debug().
					Int("status", http.StatusConflict).
					Str("code", "webhook_id_already_exists").
					Msg("webhook ID already exists")
			}

			wasRemoved, err = h.RedisClient.Persist(timeout, "oauth:github:webhook:hook_id:"+repoIDStr).Result()
			if err != nil {
				logger.Error().
					Err(err).
					Int("status", http.StatusInternalServerError).
					Str("code", "webhook_id_ttl_remove_failed").
					Msg("failed to remove webhook ID TTL")

				lib.Pretty(w, http.StatusInternalServerError, lib.Error{
					Code:    "INTERNAL_SERVER_ERROR",
					Message: "Failed to remove webhook ID TTL",
					TraceID: u.Trace,
				})
				return
			}

			if !wasRemoved {
				logger.Error().
					Int("status", http.StatusInternalServerError).
					Str("code", "webhook_id_ttl_remove_failed").
					Msg("failed to remove webhook ID TTL")

				lib.Pretty(w, http.StatusInternalServerError, lib.Error{
					Code:    "INTERNAL_SERVER_ERROR",
					Message: "Failed to remove webhook ID TTL",
					TraceID: u.Trace,
				})
				return
			}

			logger.Info().
				Int("status", http.StatusOK).
				Str("code", "github_sync_completed").
				Msg("synced GitHub data and created webhook")

			lib.Pretty(w, http.StatusOK, lib.Error{
				Code:    "OK",
				Message: "Successfully synced GitHub data",
				TraceID: u.Trace,
			})
		}
	}
}
