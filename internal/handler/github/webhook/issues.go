package webhook

import (
	"context"
	"errors"
	"net/http"
	"prolepsis/internal/auth"
	db "prolepsis/internal/db/sqlc"
	"prolepsis/internal/errlog"
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
		errlog.InternalEmptyValueError(logger, "trace_context", w, trace)
		return
	}
	timeout, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var req GitHubIssues
	err := sonic.Unmarshal(body, &req)
	if err != nil {
		errlog.ParsingError(logger, "issue_format", w, trace, err)
		return
	}

	repoIDStr := strconv.FormatInt(req.RepoInfo.ID, 10)
	hookIDStr, err := h.RedisClient.Get(timeout, "oauth:github:webhook:hook_id:"+repoIDStr).Result()
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			errlog.DeadlineExceededError(logger, "hook_id_fetch", w, trace, err)
			return
		}
		if redis.IsAuthError(err) {
			errlog.RedisAuthenticationError(logger, w, trace, err)
		}
		if errors.Is(err, redis.Nil) {
			errlog.NotFoundError(logger, "hook_id", w, trace, err)
			return
		}
		errlog.UnexpectedError(logger, "hook_id_fetch", w, trace, err)
		return
	}

	if hookIDStr == "" {
		errlog.EmptyValueError(logger, "hook_id", w, trace)
		return
	}

	if req.IssueInfo.Title == "" {
		errlog.EmptyValueError(logger, "issue_title", w, trace)
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
		errlog.InvalidError(logger, "issue_state", w, trace)
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
		errlog.ParsingError(logger, "hook_id", w, trace, err)
		return
	}

	var pgErr *pgconn.PgError

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
		if errors.Is(err, context.DeadlineExceeded) {
			errlog.DeadlineExceededError(logger, "issue_insert", w, trace, err)
			return
		}
		if errors.Is(err, pgconn.ErrConnClosed) {
			errlog.ErrConnClosed(logger, w, trace, err)
			return
		}
		if errors.As(err, &pgErr) {
			errlog.PgConnError(logger, w, trace, "issue_insert_failed", pgErr, err)
			return
		}
		errlog.UnexpectedError(logger, "issue_insert", w, trace, err)
		return
	}

	logger.Debug().
		Str("code", "issue_inset").
		Msg("successfully inserted issue")

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
		if errors.Is(err, context.DeadlineExceeded) {
			errlog.DeadlineExceededError(logger, "issue_info_insert", w, trace, err)
			return
		}
		if errors.Is(err, pgconn.ErrConnClosed) {
			errlog.ErrConnClosed(logger, w, trace, err)
			return
		}
		if errors.As(err, &pgErr) {
			errlog.PgConnError(logger, w, trace, "issue_info_insert_failed", pgErr, err)
			return
		}
		errlog.UnexpectedError(logger, "issue_info_insert", w, trace, err)
		return
	}

	if rows == 0 {
		logger.Info().
			Int("status", http.StatusOK).
			Str("code", "issue_already_exists").
			Msg("issue already exists")
		lib.Pretty(w, http.StatusOK, map[string]string{
			"status":  "OK",
			"message": "Issue already present in database",
		})
		return
	}

	logger.Debug().
		Str("code", "issue_info_inset").
		Int64("issue_id", req.IssueInfo.ID).
		Int64("issue_connected_to_repo_id", req.RepoInfo.ID).
		Str("issue_title", req.IssueInfo.Title).
		Msg("successfully insterted webhook issue")

	for _, label := range req.IssueInfo.Labels {
		err := h.Queries.InsertGitHubIssueLabel(timeout, db.InsertGitHubIssueLabelParams{
			IssueID:     req.IssueInfo.ID,
			LabelID:     label.ID,
			Name:        label.Name,
			Description: label.Description,
		})
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				errlog.DeadlineExceededError(logger, "issue_label_insert", w, trace, err)
				return
			}
			if errors.Is(err, pgconn.ErrConnClosed) {
				errlog.ErrConnClosed(logger, w, trace, err)
				return
			}
			if errors.As(err, &pgErr) {
				errlog.PgConnError(logger, w, trace, "issue_label_insert_failed", pgErr, err)
				return
			}
			errlog.UnexpectedError(logger, "issue_label_insert", w, trace, err)
			return
		}
	}
	logger.Debug().
		Str("code", "issue_label_inset").
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
			if errors.Is(err, context.DeadlineExceeded) {
				errlog.DeadlineExceededError(logger, "issue_field_value_insert", w, trace, err)
				return
			}
			if errors.Is(err, pgconn.ErrConnClosed) {
				errlog.ErrConnClosed(logger, w, trace, err)
				return
			}
			if errors.As(err, &pgErr) {
				errlog.PgConnError(logger, w, trace, "issue_field_value_insert_failed", pgErr, err)
				return
			}
			errlog.UnexpectedError(logger, "issue_field_value_insert", w, trace, err)
			return
		}

		logger.Debug().
			Str("code", "issue_field_value_inset").
			Str("field_value_id", fieldID.String()).
			Str("field_value_name", issueFieldValue.IssueFieldName).
			Msg("successfully inserted issue field value")

		if issueFieldValue.MultiSelectOptions != nil {
			for _, multiSelectOption := range *issueFieldValue.MultiSelectOptions {
				err := h.Queries.InsertGitHubIssueMultiSelectOption(timeout, db.InsertGitHubIssueMultiSelectOptionParams{
					FieldValueID: fieldID,
					ID:           multiSelectOption.MultiSelectOptionID,
					Name:         multiSelectOption.Name,
				})
				if err != nil {
					if errors.Is(err, context.DeadlineExceeded) {
						errlog.DeadlineExceededError(logger, "issue_multi_select_option_insert", w, trace, err)
						return
					}
					if errors.Is(err, pgconn.ErrConnClosed) {
						errlog.ErrConnClosed(logger, w, trace, err)
						return
					}
					if errors.As(err, &pgErr) {
						errlog.PgConnError(logger, w, trace, "issue_multi_select_option_failed", pgErr, err)
						return
					}
					errlog.UnexpectedError(logger, "issue_multi_select_option_insert", w, trace, err)
					return
				}

				logger.Debug().
					Str("code", "issue_multi_select_option_inset").
					Int64("multi_select_option_id", multiSelectOption.MultiSelectOptionID).
					Str("multi_select_option_name", multiSelectOption.Name).
					Msg("successfully inserted the issue multi secelt option")
			}
		}
	}

	logger.Info().
		Str("code", "success").
		Msg("issue and issue-related fields inserted")
	lib.Pretty(w, http.StatusOK, map[string]string{
		"reason": "success",
	})
}
