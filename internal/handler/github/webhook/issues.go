package webhook

import (
	"context"
	"errors"
	"net/http"
	"prolepsis/internal/auth"
	db "prolepsis/internal/db/sqlc"
	"prolepsis/internal/lib"
	"strconv"
	"time"

	"github.com/bytedance/sonic"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

func (h *WebhookHandler) WebhookIssues(w http.ResponseWriter, r *http.Request, body []byte, deliveryID string) {
	logger := zerolog.Ctx(r.Context()).With().Str("event", "issues").Logger()
	trace, ok := auth.GetTrace(r.Context())
	if !ok {
		logger.Error().
			Int("status", http.StatusInternalServerError).
			Str("cause", "missing_tracing_context").
			Msg("handler invoked without trace ID in context")

		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "An internal server error occurred",
			Details: map[string]string{
				"reason": "request context pipeline uninitialized",
			},
		})
		return
	}
	timeout, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var req GitHubIssues
	err := sonic.Unmarshal(body, &req)
	if err != nil {
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("cause", "failed_to_unmarshal").
			Msg("failed to unmarshal the given body")
		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "Unrevognized error while trying to unmarshal the given body",
			TraceID: trace,
		})
		return
	}

	repoIDStr := strconv.FormatInt(req.RepoInfo.ID, 10)
	hookIDStr, err := h.RedisClient.Get(timeout, "oauth:github:webhook:hook_id:"+repoIDStr).Result()
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			logger.Error().
				Err(err).
				Int("status", http.StatusRequestTimeout).
				Str("cause", "timeout").
				Msg("failed to fetch the webhook id from redis due to timeout")
			lib.Pretty(w, http.StatusRequestTimeout, lib.Error{
				Code:    "REQUEST_TIMEOUT",
				Message: "Timedout while trying to get the webhook if",
				TraceID: trace,
			})
			return
		}
		if errors.Is(err, redis.Nil) {
			logger.Warn().
				Err(err).
				Int("status", http.StatusNotFound).
				Str("cause", "missing_webhook_id").
				Int64("repo_id", req.RepoInfo.ID).
				Str("full_name", req.RepoInfo.FullName).
				Msg("missing webhook id inside redis")
			lib.Pretty(w, http.StatusNotFound, lib.Error{
				Code:    "NOT_FOUND",
				Message: "Failed due to missing webhook id",
				TraceID: trace,
			})
			return
		}
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("cause", "unrecognized").
			Msg("unrecognized error while trying to check for the webhook id inside redis")
		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "Failed due to unrecognized error while trying to check for the webhook existence inside redis",
			TraceID: trace,
		})
		return
	}

	if hookIDStr == "" {
		logger.Warn().
			Int("status", http.StatusBadRequest).
			Str("cause", "empty_webhook_id").
			Msg("failed due to empty webhook id")
		lib.Pretty(w, http.StatusBadRequest, lib.Error{
			Code:    "BAD_REQUEST",
			Message: "Empty webhook id, cannot continue",
			TraceID: trace,
		})
		return
	}

	if req.IssueInfo.Title == "" {
		logger.Error().
			Int("status", http.StatusBadRequest).
			Str("cause", "missing_issue_title").
			Msg("the given payload's issue title is missing")

		lib.Pretty(w, http.StatusBadRequest, lib.Error{
			Code:    "BAD_REQUEST",
			Message: "Invalid payload, issue title is missing",
			TraceID: trace,
		})

		return
	}

	assignee := req.AssigneeInfo

	var assigneeID *int64
	var assigneeName *string
	var assigneeType *string
	if assignee != nil {
		assigneeID = &req.AssigneeInfo.ID
		assigneeName = &req.AssigneeInfo.Name
		assigneeType = &req.AssigneeInfo.Type
	}

	milestone := req.IssueInfo.Milestone
	var milestoneDescription *string
	var milestoneDueOn *string
	var milestoneState *string
	var milestoneTitle *string
	if milestone != nil {
		milestoneDescription = req.IssueInfo.Milestone.Description
		milestoneDueOn = req.IssueInfo.Milestone.DueOn
		milestoneState = &req.IssueInfo.Milestone.State
		milestoneTitle = &req.IssueInfo.Milestone.Title
	}

	positiveReactions := req.IssueInfo.Reactions.Hooray + req.IssueInfo.Reactions.PositiveReaction + req.IssueInfo.Reactions.Heart + req.IssueInfo.Reactions.Laugh + req.IssueInfo.Reactions.Rocket
	negativeReactions := req.IssueInfo.Reactions.Confused + req.IssueInfo.Reactions.NegativeReaction

	if req.IssueInfo.State != "open" && req.IssueInfo.State != "closed" {
		logger.Error().
			Int("status", http.StatusBadRequest).
			Str("cause", "invalid_issue_state").
			Str("given_state", req.IssueInfo.State).
			Msg("the given payload's state is not 'closed' or 'open'")
		lib.Pretty(w, http.StatusBadRequest, lib.Error{
			Code:    "BAD_REQUEST",
			Message: "Invalid payload, state is malformed",
			TraceID: trace,
		})
		return
	}

	issueEventIDRaw := uuid.New()
	issueEventID := pgtype.UUID{Bytes: issueEventIDRaw, Valid: true}
	var closedAt pgtype.Timestamptz
	if req.IssueInfo.ClosedAt != nil {
		closedAt = pgtype.Timestamptz{Time: *req.IssueInfo.ClosedAt, Valid: true}
	}

	hookID, err := strconv.ParseInt(hookIDStr, 10, 64)
	if err != nil {
		logger.Warn().
			Err(err).
			Int("status", http.StatusBadRequest).
			Str("cause", "failed_hook_id_parsing").
			Msg("failed to parse the webhook id from string to int64")
		lib.Pretty(w, http.StatusBadRequest, lib.Error{
			Code:    "BAD_REQUEST",
			Message: "Failed webhook id parsing",
			TraceID: trace,
		})
		return
	}

	err = h.Queries.InsertGitHubIssue(timeout, db.InsertGitHubIssueParams{
		ID:           issueEventID,
		GithubRepoID: req.RepoInfo.ID,
		GithubUserID: req.RepoInfo.OwnerID.ID,
		HookID:       hookID,
		EventAction:  req.Action,
		AssigneeID:   assigneeID,
		AssigneeName: assigneeName,
		AssigneeType: assigneeType,
		SenderID:     req.Sender.ID,
		SenderName:   req.Sender.Login,
		SenderType:   req.Sender.Type,
	})
	if err != nil {
		if errors.Is(err, pgconn.ErrConnClosed) {
			logger.Error().
				Err(err).
				Int("status", http.StatusRequestTimeout).
				Str("cause", "timeout").
				Msg("failed to insert the webhook issue event inside the database due to timeout")
			lib.Pretty(w, http.StatusRequestTimeout, lib.Error{
				Code:    "REQUEST_TIMEOUT",
				Message: "Failed to insert the webhook issue event due to timeout",
				TraceID: trace,
			})
			return
		}
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("cause", "unrecognized").
			Msg("failed to insert the webhook issue event inside the database due to unrecognized error")
		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "Unrecognized error while trying to insert the webhook issue event inside the database",
			TraceID: trace,
		})
		return
	}

	logger.Info().
		Str("cause", "success").
		Msg("successfully inserted all the info inside the database")

	var typeName *string
	var typeDescription *string
	if req.IssueInfo.Type != nil {
		typeName = &req.IssueInfo.Type.Name
		typeDescription = req.IssueInfo.Type.Description
	}

	var userID *int64
	var userName *string
	var userType *string
	if req.IssueInfo.User != nil {
		userID = &req.IssueInfo.User.ID
		userName = &req.IssueInfo.User.Name
		userType = &req.IssueInfo.User.Type
	}

	var innerAssigneeID *int64
	var innerAssigneeName *string
	var innerAssigneeType *string
	if req.IssueInfo.Assignee != nil {
		innerAssigneeID = &req.IssueInfo.Assignee.ID
		innerAssigneeName = &req.IssueInfo.Assignee.Name
		innerAssigneeType = &req.IssueInfo.Assignee.Type
	}

	rows, err := h.Queries.InsertGitHubIssueInfo(timeout, db.InsertGitHubIssueInfoParams{
		RelatedID:                     issueEventID,
		IssueID:                       req.IssueInfo.ID,
		AuthorAssociation:             req.IssueInfo.AuthAssociation,
		Body:                          req.IssueInfo.Body,
		Comments:                      int32(req.IssueInfo.Comments),
		Draft:                         req.IssueInfo.Draft,
		CreatedAt:                     pgtype.Timestamptz{Time: req.IssueInfo.CreatedAt, Valid: true},
		DeletedAt:                     closedAt,
		Locked:                        req.IssueInfo.Locked,
		MilestoneDescription:          milestoneDescription,
		MilestoneDueOn:                milestoneDueOn,
		MilestoneState:                milestoneState,
		MilestoneTitle:                milestoneTitle,
		Number:                        int32(req.IssueInfo.Number),
		PositiveReactions:             int32(positiveReactions),
		NegativeReactions:             int32(negativeReactions),
		ReactionsTotalCount:           int32(req.IssueInfo.Reactions.TotalCount),
		State:                         req.IssueInfo.State,
		StateReason:                   req.IssueInfo.StateReason,
		SubIssueTotal:                 int32(req.IssueInfo.SubIssuesSummary.Total),
		SubIssueCompleted:             int32(req.IssueInfo.SubIssuesSummary.Completed),
		SubIssuePercentCompleted:      int32(req.IssueInfo.SubIssuesSummary.PercentCompleted),
		IssueDependencyTotalBlockedBy: int32(req.IssueInfo.IssueDependenciesSummary.TotalBlockedBy),
		IssueDependencyTotalBlocking:  int32(req.IssueInfo.IssueDependenciesSummary.TotalBlocking),
		Title:                         req.IssueInfo.Title,
		TypeName:                      typeName,
		TypeDescription:               typeDescription,
		UpdatedAt:                     pgtype.Timestamptz{Time: req.IssueInfo.UpdatedAt, Valid: true},
		IssueCreatedBy:                userID,
		CreatorName:                   userName,
		CreatorType:                   userType,
		AssigneeID:                    innerAssigneeID,
		AssigneeName:                  innerAssigneeName,
		AssigneeType:                  innerAssigneeType,
	})
	if err != nil {
		if errors.Is(err, pgconn.ErrConnClosed) {
			logger.Error().
				Err(err).
				Int("status", http.StatusRequestTimeout).
				Str("cause", "timeout").
				Msg("failed to insert the issue info inside the database due to timeout")
			lib.Pretty(w, http.StatusRequestTimeout, lib.Error{
				Code:    "REQUEST_TIMEOUT",
				Message: "Timedout while trying to insert the issue info inside the database",
				TraceID: trace,
			})
			return
		}
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("cause", "unrecognized").
			Msg("failed to insert the issue info inside the database due to an unrecognized error")
		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "Unrecognized error while trying to insert the issue info inside the database",
			TraceID: trace,
		})
		return
	}

	if rows == 0 {
		logger.Info().
			Int("status", http.StatusOK).
			Str("cause", "already_existing_issue").
			Msg("issue already exists in database, skipping insert")

		lib.Pretty(w, http.StatusOK, map[string]string{
			"status":  "already_exists",
			"message": "Issue already present in database",
		})
		return
	}

	logger.Debug().
		Str("cause", "issue_info_successfully_inserted").
		Int64("issue_id", req.IssueInfo.ID).
		Int64("issue_connected_to_repo_id", req.RepoInfo.ID).
		Str("issue_title", req.IssueInfo.Title).
		Msg("successfully insterted the webhook issue inside the database")

	for _, label := range req.IssueInfo.Labels {
		err := h.Queries.InsertGitHubIssueLabel(timeout, db.InsertGitHubIssueLabelParams{
			IssueID:     req.IssueInfo.ID,
			LabelID:     label.ID,
			Name:        label.Name,
			Description: label.Description,
		})
		if err != nil {
			if errors.Is(err, pgconn.ErrConnClosed) {
				logger.Error().
					Err(err).
					Int("status", http.StatusRequestTimeout).
					Str("cause", "timeout").
					Msg("failed to insert the github issue label due to timeout")
				lib.Pretty(w, http.StatusRequestTimeout, lib.Error{
					Code:    "REQUEST_TIMEOUT",
					Message: "Timedout while trying to insert the github issue label inside the database",
					TraceID: trace,
				})
			}
			logger.Error().
				Err(err).
				Int("status", http.StatusInternalServerError).
				Str("cause", "unrecognized").
				Msg("failed to insert the github issue label due to timeout")
			lib.Pretty(w, http.StatusInternalServerError, lib.Error{
				Code:    "INTERNAL_SERVER_ERROR",
				Message: "Hit an unrecognized error while trying to insert the github issue label",
				TraceID: trace,
			})
			return
		}
	}
	logger.Debug().
		Str("cause", "labels_added_successfully").
		Msg("successfully added all the labels")

	for _, issueFieldValue := range req.IssueInfo.IssueFieldValues {
		singleSelectOption := issueFieldValue.SingleSelectOption

		var singleSOID *int64
		var singleSOName *string
		if singleSelectOption != nil {
			singleSOID = &singleSelectOption.ID
			singleSOName = &singleSelectOption.Name
		}

		fieldID, err := h.Queries.InsertGitHubIssueFieldValue(timeout, db.InsertGitHubIssueFieldValueParams{
			IssueID:                req.IssueInfo.ID,
			IssueFieldName:         issueFieldValue.IssueFieldName,
			DataType:               issueFieldValue.DataType,
			Value:                  issueFieldValue.Value,
			SingleSelectOptionID:   singleSOID,
			SingleSelectOptionName: singleSOName,
		})
		if err != nil {
			if errors.Is(err, pgconn.ErrConnClosed) {
				logger.Error().
					Err(err).
					Int("status", http.StatusRequestTimeout).
					Str("cause", "timeout").
					Msg("failed to insert the github issue field value due to timeout")
				lib.Pretty(w, http.StatusRequestTimeout, lib.Error{
					Code:    "REQUEST_TIMEOUT",
					Message: "Timedout while trying to insert the github issue field value inside the database",
					TraceID: trace,
				})
				return
			}
			logger.Error().
				Err(err).
				Int("status", http.StatusInternalServerError).
				Str("cause", "unrecognized").
				Msg("failed to insert the github issue field value due to unrecognized error")
			lib.Pretty(w, http.StatusInternalServerError, lib.Error{
				Code:    "INTERNAL_SERVER_ERROR",
				Message: "Unrecognized error while trying to insert the github issue field value inside the database",
				TraceID: trace,
			})
			return
		}

		logger.Debug().
			Str("cause", "field_value_inserted_successfully").
			Str("field_value_id", fieldID.String()).
			Str("field_value_name", issueFieldValue.IssueFieldName).
			Msg("successfully inserted the field value inside the database")

		if issueFieldValue.MultiSelectOptions != nil {
			for _, multiSelectOption := range *issueFieldValue.MultiSelectOptions {
				err := h.Queries.InsertGitHubIssueMultiSelectOption(timeout, db.InsertGitHubIssueMultiSelectOptionParams{
					FieldValueID: fieldID,
					ID:           multiSelectOption.MultiSelectOptionID,
					Name:         multiSelectOption.Name,
				})
				if err != nil {
					if errors.Is(err, pgconn.ErrConnClosed) {
						logger.Error().
							Err(err).
							Int("status", http.StatusRequestTimeout).
							Str("cause", "timeout").
							Int64("multi_select_option_id", multiSelectOption.MultiSelectOptionID).
							Str("multi_select_option_name", multiSelectOption.Name).
							Msg("failed to insert the github multi select option due to timeout")
						lib.Pretty(w, http.StatusRequestTimeout, lib.Error{
							Code:    "REQUEST_TIMEOUT",
							Message: "Timedout while trying to insert the github multi select option inside the database",
							TraceID: trace,
						})
						return
					}
					logger.Error().
						Err(err).
						Int("status", http.StatusInternalServerError).
						Str("cause", "unrecognized").
						Int64("multi_select_option_id", multiSelectOption.MultiSelectOptionID).
						Str("multi_select_option_name", multiSelectOption.Name).
						Msg("failed to insert the github multi select option due to unrecognized error")
					lib.Pretty(w, http.StatusInternalServerError, lib.Error{
						Code:    "INTERNAL_SERVER_ERROR",
						Message: "Unrecognized error while trying to insert the github multi select option inside the database",
						TraceID: trace,
					})
					return
				}

				logger.Debug().
					Str("cause", "multi_select_option_inserted_successfully").
					Int64("multi_select_option_id", multiSelectOption.MultiSelectOptionID).
					Str("multi_select_option_name", multiSelectOption.Name).
					Msg("successfully inserted the multi_select_option inside the database")
			}
		}
	}

	logger.Debug().
		Str("cause", "all_issue_fields_inserted_successfully").
		Msg("successfully inserted all the issue fields and the multi select options")
	lib.Pretty(w, http.StatusOK, map[string]string{
		"reason": "success",
	})
}
