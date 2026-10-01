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
	timeout, cancelGeneral := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancelGeneral()
	timeoutRepo, cancelRepo := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancelRepo()
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

	reqU, err := http.NewRequestWithContext(timeout, "GET", "https://api.github.com/user", nil)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			logger.Error().
				Err(err).
				Int("status", http.StatusRequestTimeout).
				Str("cause", "timeout").
				Msg("timeout while trying to generate a request to the github/user endpoint")
			lib.Pretty(w, http.StatusRequestTimeout, lib.Error{
				Code:    "REQUEST_TIMEOUT",
				Message: "Failed to create user-related request due to timeout",
				TraceID: u.Trace,
			})
			return
		}
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("cause", "unrecognized").
			Msg("unrecognized error while trying to create a request to the endpoint github/user")
		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "Failed to create an user-related request due to unrecognized error",
			TraceID: u.Trace,
		})
		return
	}
	reqU.Header.Set("Authorization", "Bearer "+token.AccessToken)

	respUser, err := h.HTTP_Client.Do(reqU)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			logger.Error().
				Err(err).
				Int("status", http.StatusRequestTimeout).
				Str("cause", "timeout").
				Msg("timedout while trying to get the response from github/user")
			lib.Pretty(w, http.StatusRequestTimeout, lib.Error{
				Code:    "REQUEST_TIMEOUT",
				Message: "Timeout while trying to get response from github/user",
				TraceID: u.Trace,
			})
			return
		}
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("cause", "unrecognized").
			Msg("failed to fetch the response from github/user due to unrecognized error")
		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "Unrecognized error while trying to fetch the response from github/user",
			TraceID: u.Trace,
		})
		return
	}

	if respUser.StatusCode != 200 {
		logger.Error().
			Int("status", respUser.StatusCode).
			Msg("hit an unrecognized error while trying to get the response from github/user")
		lib.Pretty(w, respUser.StatusCode, lib.Error{
			Code:    respUser.Status,
			Message: "The given status for the response from github/user is not 200",
			TraceID: u.Trace,
		})
	}

	var reqUser GitHubUser
	if err := sonic.ConfigDefault.NewDecoder(respUser.Body).Decode(&reqUser); err != nil {
		logger.Error().
			Int("status", http.StatusInternalServerError).
			Str("cause", "could_not_decode").
			Msg("could not decode the given request about the user")
		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "Failed to decode the given information about the user",
			TraceID: u.Trace,
		})
		return
	}
	defer func() {
		err = respUser.Body.Close()
		if err != nil {
			logger.Error().
				Err(err).
				Int("status", http.StatusInternalServerError).
				Str("cause", "failed_response_closure").
				Msg("failed to close the user response conncection")
			lib.Pretty(w, http.StatusInternalServerError, lib.Error{
				Code:    "INTERNAL_SERVER_ERROR",
				Message: "Unrecognized error caused user response connection issues, couldn't close body",
				TraceID: u.Trace,
			})
			return
		}
	}()

	totalRepos := reqUser.GitHubPrivateRepos + reqUser.GitHubPublicRepos
	if totalRepos == 0 {
		logger.Warn().
			Int("status", http.StatusNotFound).
			Str("cause", "no_repositories_found").
			Msg("no repository found")
		lib.Pretty(w, http.StatusNotFound, lib.Error{
			Code:    "NOT_FOUND",
			Message: "The given github account contains no repositories",
			Details: map[string]string{
				"reason": "empty github account",
				"fix":    "create a repository to be considered",
			},
			TraceID: u.Trace,
		})
		return
	}

	exists, err := h.Queries.UserExistsInsideGitHub(timeout, u.ID)
	if err != nil {
		if errors.Is(err, pgconn.ErrConnClosed) {
			logger.Error().
				Err(err).
				Int("status", http.StatusRequestTimeout).
				Str("cause", "timeout").
				Msg("timedout while trying to check if the user's github account is already inside the database")
			lib.Pretty(w, http.StatusRequestTimeout, lib.Error{
				Code:    "REQUEST_TIMEOUT",
				Message: "Hit timeout while trying to check if the user's github account exists",
			})
			return
		}
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("cause", "unrecognized").
			Msg("unrecognized error while trying to check for user's github accout existence inside the database")
		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "Unrecognized error while checking user's existence - github",
		})
		return
	}

	if exists {
		logger.Debug().
			Int("status", http.StatusConflict).
			Str("cause", "github_user_exists").
			Str("username", reqUser.GitHubUsername).
			Int64("github_id", reqUser.GitHubID).
			Msg("the user already is registred in the database - updating the database data")
	}

	err = h.Queries.InsertGitHubUserInfo(timeout, db.InsertGitHubUserInfoParams{
		UserID:            u.ID,
		GithubID:          reqUser.GitHubID,
		Name:              reqUser.GitHubUsername,
		AvatarUrl:         reqUser.AvatarURL,
		HtmlUrl:           reqUser.GitHubHTMLURL,
		Company:           reqUser.GitHubCompany,
		Email:             reqUser.GitHubEmail,
		Hireable:          reqUser.GitHubHireable,
		Bio:               reqUser.GitHubBio,
		Followers:         reqUser.GitHubFollowers,
		TotalPublicRepos:  reqUser.GitHubPublicRepos,
		TotalPrivateRepos: reqUser.GitHubPrivateRepos,
		TotalRepos:        totalRepos,
		CreatedAt:         pgtype.Timestamptz{Time: reqUser.GitHubCreatedAt, Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgconn.ErrConnClosed) {
			logger.Error().
				Err(err).
				Int("status", http.StatusRequestTimeout).
				Str("cause", "timeout").
				Msg("timedout while trying to insert the user's info inside the database")
			lib.Pretty(w, http.StatusRequestTimeout, lib.Error{
				Code:    "REQUEST_TIMEOUT",
				Message: "Timeout whle trying to insert user's info inside the database",
				TraceID: u.Trace,
			})
			return
		}
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("cause", "unrecognized").
			Msg("coulndn't insert user's info inside the database due to unrecognized error")
		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "Failed to insert user's info inside the database",
			TraceID: u.Trace,
		})
		return
	}

	for repo := range totalRepos {
		var repos []GitHubRepo
		url := fmt.Sprintf("https://api.github.com/user/repos?page=%d&per_page=1", repo+1)
		reqR, err := http.NewRequestWithContext(timeoutRepo, http.MethodGet, url, nil)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				logger.Error().
					Err(err).
					Int("status", http.StatusRequestTimeout).
					Str("cause", "timeout").
					Int16("repo_number", repo).
					Msg("couldn't retrieve the info about user's repositories due to timeout")
				lib.Pretty(w, http.StatusRequestTimeout, lib.Error{
					Code:    "REQUEST_TIMEOUT",
					Message: "Timedout while trying to fetch info about all the repositories",
					TraceID: u.Trace,
				})
				return
			}
			logger.Error().
				Err(err).
				Int("status", http.StatusInternalServerError).
				Str("cause", "unrecognized").
				Int16("repo_number", repo).
				Msg("unrecognized error while trying to retrieve info about the repositories")
			lib.Pretty(w, http.StatusInternalServerError, lib.Error{
				Code:    "INTERNAL_SERVER_ERROR",
				Message: "Failed to get the repository info",
				TraceID: u.Trace,
			})
			return
		}
		reqR.Header.Set("Authorization", "Bearer "+token.AccessToken)

		respR, err := h.HTTP_Client.Do(reqR)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				logger.Error().
					Err(err).
					Int("status", http.StatusRequestTimeout).
					Str("cause", "timeout").
					Msg("timed out while requesting the GitHub repo info")
				lib.Pretty(w, http.StatusRequestTimeout, lib.Error{
					Code:    "REQUEST_TIMEOUT",
					Message: "Failed to finish the request for github's repo info due to timeout",
					TraceID: u.Trace,
				})
				return
			}
			logger.Error().
				Err(err).
				Int("status", http.StatusInternalServerError).
				Str("cause", "unrecognized").
				Msg("failed to request the GitHub repo info due to unrecognized error")
			lib.Pretty(w, http.StatusInternalServerError, lib.Error{
				Code:    "INTERNAL_SERVER_ERROR",
				Message: "Failed to request the GitHub user profile",
				TraceID: u.Trace,
			})
			return
		}

		if respR.StatusCode != http.StatusOK {
			logger.Error().
				Int("status", respR.StatusCode).
				Msg("unexpected status code from github")
			lib.Pretty(w, respR.StatusCode, lib.Error{
				Code:    "GITHUB_API_ERROR",
				Message: "Failed to fetch GitHub repository",
				TraceID: u.Trace,
			})
			return
		}

		err = sonic.ConfigDefault.NewDecoder(respR.Body).Decode(&repos)
		if err != nil {
			logger.Error().
				Err(err).
				Int("status", http.StatusInternalServerError).
				Str("cause", "could_not_decode").
				Msg("could not decode the given repository info")
			lib.Pretty(w, http.StatusInternalServerError, lib.Error{
				Code:    "INTERNAL_SERVER_ERROR",
				Message: "Failed to decode the repository info",
				TraceID: u.Trace,
			})
			return
		}
		err = respR.Body.Close()
		if err != nil {
			logger.Error().
				Err(err).
				Int("status", http.StatusInternalServerError).
				Str("cause", "failed_response_closure").
				Msg("failed to close the respository response conncection")
			lib.Pretty(w, http.StatusInternalServerError, lib.Error{
				Code:    "INTERNAL_SERVER_ERROR",
				Message: "Unrecognized error caused repository response connection issues, couldn't close body",
				TraceID: u.Trace,
			})
			return
		}

		reqRepo := repos[0]

		if reqRepo.RepoForked {
			logger.Debug().
				Int64("repo_id", reqRepo.RepoID).
				Str("repoName", reqRepo.RepoName).
				Msg("skipped forked repository")
			continue
		}

		exists, err := h.Queries.RepoExistsInsideGitHub(timeoutRepo, reqRepo.RepoID)
		if err != nil {
			if errors.Is(err, pgconn.ErrConnClosed) {
				logger.Error().
					Err(err).
					Int("status", http.StatusRequestTimeout).
					Str("cause", "timeout").
					Msg("timedout while trying to insert the user's repo inside the database")
				lib.Pretty(w, http.StatusRequestTimeout, lib.Error{
					Code:    "REQUEST_TIMEOUT",
					Message: "Timeout whle trying to check if user's repo is inside the database",
					TraceID: u.Trace,
				})
				return
			}
			logger.Error().
				Err(err).
				Int("status", http.StatusInternalServerError).
				Str("cause", "unrecognized").
				Msg("coulndn't check user's repo inside the database due to unrecognized error")
			lib.Pretty(w, http.StatusInternalServerError, lib.Error{
				Code:    "INTERNAL_SERVER_ERROR",
				Message: "Failed to check user's repo inside the database",
				TraceID: u.Trace,
			})
			return
		}

		if exists {
			logger.Debug().
				Int64("repo_id", reqRepo.RepoID).
				Str("repo_name", reqRepo.RepoName).
				Msg("repository already exists in database, updating database info about it (if changes applied)")
		}

		var licenseKey *string
		if reqRepo.RepoLicense != nil {
			licenseKey = &reqRepo.RepoLicense.Key
			exists, err := h.Queries.LicenseExistsInsideGitHub(timeout, *licenseKey)
			if err != nil {
				if errors.Is(err, pgconn.ErrConnClosed) {
					logger.Error().
						Err(err).
						Int("status", http.StatusRequestTimeout).
						Str("cause", "timeout").
						Msg("hit timeout while trying to check if the given license exists inside the database")
					lib.Pretty(w, http.StatusRequestTimeout, lib.Error{
						Code:    "REQUEST_TIMEOUT",
						Message: "Timeout while checking if the given license exists inside the database",
						TraceID: u.Trace,
					})
					return
				}
				logger.Error().
					Err(err).
					Int("status", http.StatusInternalServerError).
					Str("cause", "unrecognized").
					Msg("unrecognized error while trying to check if the given license exists inside the database")
				lib.Pretty(w, http.StatusInternalServerError, lib.Error{
					Code:    "INTERNAL_SERVER_ERROR",
					Message: "Unrecognized error while checking license existence",
					TraceID: u.Trace,
				})
				return
			}
			if !exists {
				err = h.Queries.InsertGitHubLicenseInfo(timeout, db.InsertGitHubLicenseInfoParams{
					LicenseKey: reqRepo.RepoLicense.Key,
					Name:       reqRepo.RepoLicense.Name,
					SpdxID:     reqRepo.RepoLicense.SpdxID,
					Url:        reqRepo.RepoLicense.URL,
					NodeID:     reqRepo.RepoLicense.NodeID,
				})
				if err != nil {
					if errors.Is(err, pgconn.ErrConnClosed) {
						logger.Error().
							Err(err).
							Int("status", http.StatusRequestTimeout).
							Str("cause", "timeout").
							Msg("failed to insert repo's license inside the database due to timeout")
						lib.Pretty(w, http.StatusRequestTimeout, lib.Error{
							Code:    "REQUEST_TIMEOUT",
							Message: "Timeout while trying to insert repository's license inside the database",
							TraceID: u.Trace,
						})
						return
					}
					logger.Error().
						Err(err).
						Int("status", http.StatusInternalServerError).
						Str("cause", "unrecognized").
						Msg("failed to insert repo's license inside the database")
					lib.Pretty(w, http.StatusInternalServerError, lib.Error{
						Code:    "INTERNAL_SERVER_ERROR",
						Message: "Couldn't insert repository's license info inside the database",
						TraceID: u.Trace,
					})
					return
				}
			}
		}

		err = h.Queries.InsertGitHubRepoInfo(timeoutRepo, db.InsertGitHubRepoInfoParams{
			GithubID:     reqUser.GitHubID,
			RepoID:       reqRepo.RepoID,
			Name:         reqRepo.RepoName,
			HtmlUrl:      reqRepo.RepoHTMLURL,
			Description:  reqRepo.RepoDescription,
			CreatedAt:    pgtype.Timestamptz{Time: reqRepo.RepoCreatedAt, Valid: true},
			PushedAt:     pgtype.Timestamptz{Time: reqRepo.RepoPushedAt, Valid: true},
			Stars:        reqRepo.RepoStars,
			Watches:      reqRepo.RepoWatches,
			ForksCount:   reqRepo.RepoForksCount,
			MainLanguage: reqRepo.RepoLanguage,
			LicenseKey:   licenseKey,
			Topic:        reqRepo.RepoTopics,
			Archived:     reqRepo.RepoArchived,
			Visibility:   reqRepo.RepoVisibility,
		})

		if err != nil {
			if errors.Is(err, pgconn.ErrConnClosed) {
				logger.Error().
					Err(err).
					Int("status", http.StatusRequestTimeout).
					Str("cause", "timeout").
					Msg("failed to insert user's repo inside the database due to timeout")
				lib.Pretty(w, http.StatusRequestTimeout, lib.Error{
					Code:    "REQUEST_TIMEOUT",
					Message: "Timeout while trying to insert user's repository inside the database",
					TraceID: u.Trace,
				})
				return
			}
			logger.Error().
				Err(err).
				Int("status", http.StatusInternalServerError).
				Str("cause", "unrecognized").
				Msg("failed to insert user's repository inside the database")
			lib.Pretty(w, http.StatusInternalServerError, lib.Error{
				Code:    "INTERNAL_SERVER_ERROR",
				Message: "Couldn't insert user's repository info inside the database",
				TraceID: u.Trace,
			})
			return
		}

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
		reqW, err := http.NewRequestWithContext(timeoutRepo, http.MethodPost, webhookURL, body)
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

		_, err = h.RedisClient.Set(timeoutRepo, "oauth:github:webhook:secret:"+repoIDStr, secret, 0).Result()
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

		wasRemoved, err := h.RedisClient.Persist(timeoutRepo, "oauth:github:webhook:secret:"+repoIDStr).Result()
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

		does_not_exists, err := h.RedisClient.SetNX(timeoutRepo, "oauth:github:webhook:hook_id:"+repoIDStr, reqWeb.ID, 0).Result()
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
			continue
		}

		wasRemoved, err = h.RedisClient.Persist(timeoutRepo, "oauth:github:webhook:hook_id:"+repoIDStr).Result()
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
