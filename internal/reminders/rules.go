package reminders

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/jnaranj09/open-customer/internal/db"
	"github.com/jnaranj09/open-customer/internal/db/sqlc"
	"github.com/jnaranj09/open-customer/internal/platform/archive"
	"github.com/jnaranj09/open-customer/internal/platform/ids"
	"github.com/jnaranj09/open-customer/internal/platform/nullval"
	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

const maxOffsetDays = 365

// ListRules returns every rule, archived ones included, ordered by creation time and id. The slice
// is never nil.
func (m *Module) ListRules(ctx context.Context, q db.DBTX) ([]Rule, error) {
	rows, err := sqlc.New(q).ListRules(ctx)
	if err != nil {
		return nil, fmt.Errorf("list rules: %w", err)
	}
	out := make([]Rule, 0, len(rows))
	for _, r := range rows {
		out = append(out, ruleFrom(sqlc.ReminderRule(r)))
	}
	return out, nil
}

// CreateRule validates in (every bad field is reported) and stores the rule. The template, service
// and return service must exist and be active. A come-back rule with no service is a general rule
// and takes no return service. AppliesFrom is now, or nil with IncludeOverdue.
func (m *Module) CreateRule(ctx context.Context, q db.DBTX, in RuleInput) (Rule, error) {
	queries := sqlc.New(q)
	offset, err := checkRuleInput(ctx, queries, in)
	if err != nil {
		return Rule{}, err
	}
	id, err := ids.New()
	if err != nil {
		return Rule{}, fmt.Errorf("new rule id: %w", err)
	}
	now := m.clock.Now()
	var appliesFrom *time.Time
	if !in.IncludeOverdue {
		appliesFrom = &now
	}
	if err := queries.InsertRule(ctx, sqlc.InsertRuleParams{
		ID: id, Kind: string(in.Kind), ServiceID: in.ServiceID, ReturnServiceID: in.ReturnServiceID,
		OffsetDays: offset, TemplateID: in.TemplateID, AppliesFrom: appliesFrom, Now: now,
	}); err != nil {
		return Rule{}, fmt.Errorf("insert rule: %w", err)
	}
	return Rule{
		ID: id, Kind: in.Kind, ServiceID: in.ServiceID, ReturnServiceID: in.ReturnServiceID,
		OffsetDays: in.OffsetDays, TemplateID: in.TemplateID, AppliesFrom: appliesFrom, CreatedAt: now,
	}, nil
}

// checkRuleInput checks a new rule (every bad field is reported, as problem.Validation) and returns
// its offset as the column type. The rule preview checks its draft with it too.
func checkRuleInput(ctx context.Context, queries *sqlc.Queries, in RuleInput) (int32, error) {
	var errs []problem.FieldError
	if _, err := ParseKind(string(in.Kind)); err != nil {
		return 0, err
	}
	offset, offsetMsg := offsetDays(in.Kind, in.OffsetDays)
	if offsetMsg != "" {
		errs = append(errs, problem.FieldError{Field: "offset_days", Message: offsetMsg})
	}
	switch in.Kind {
	case KindBefore:
		if in.ServiceID.Valid {
			errs = append(errs, problem.FieldError{Field: "service_id", Message: problem.MsgServiceNotAllowed})
		}
		if in.ReturnServiceID.Valid {
			errs = append(errs, problem.FieldError{Field: "return_service_id", Message: problem.MsgServiceNotAllowed})
		}
	case KindComeBack:
		if !in.ServiceID.Valid && in.ReturnServiceID.Valid {
			errs = append(errs, problem.FieldError{Field: "return_service_id", Message: problem.MsgGeneralNoReturn})
		}
	}
	if err := checkActive(ctx, &errs, "template_id", problem.MsgTemplateUnavailable, in.TemplateID, queries.GetTemplateState); err != nil {
		return 0, err
	}
	if in.Kind == KindComeBack {
		if in.ServiceID.Valid {
			if err := checkActive(ctx, &errs, "service_id", problem.MsgServiceUnavailable, in.ServiceID.UUID, queries.GetServiceState); err != nil {
				return 0, err
			}
		}
		if in.ReturnServiceID.Valid {
			if err := checkActive(ctx, &errs, "return_service_id", problem.MsgServiceUnavailable, in.ReturnServiceID.UUID, queries.GetServiceState); err != nil {
				return 0, err
			}
		}
	}
	if len(errs) > 0 {
		return 0, problem.Validation{Errors: errs}
	}
	return offset, nil
}

// UpdateRule changes the given fields of one rule (every bad field is reported). An unknown id is
// problem.NotFound, before any field check. A change that can make new items appear asks the
// include-overdue question: it sets AppliesFrom to now, or to nil with IncludeOverdue. The changes
// that ask are: a new offset, a new effective return service (the return service, or the rule's own
// service when none), restoring an archived rule, a new template while the stored one is archived,
// and a new return service while the stored effective one is archived. Any other change keeps
// AppliesFrom.
//
// The handover (reminders-and-replies design, "Handover"): archiving a specific come-back rule that
// could send writes HandoverFrom, now or nil with IncludeOverdue; restoring one that can then send
// clears it. Any other change keeps it.
func (m *Module) UpdateRule(ctx context.Context, q db.DBTX, id uuid.UUID, p RulePatch) (Rule, error) {
	queries := sqlc.New(q)
	row, err := queries.GetRule(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Rule{}, problem.NotFound{}
	}
	if err != nil {
		return Rule{}, fmt.Errorf("get rule: %w", err)
	}
	kind := Kind(row.Kind)
	var errs []problem.FieldError
	var offset int32
	if p.OffsetDays != nil {
		var msg string
		if offset, msg = offsetDays(kind, *p.OffsetDays); msg != "" {
			errs = append(errs, problem.FieldError{Field: "offset_days", Message: msg})
		}
	}
	newReturn := nullval.Ptr(p.ReturnServiceID)
	switch {
	case kind == KindBefore && newReturn != nil:
		errs = append(errs, problem.FieldError{Field: "return_service_id", Message: problem.MsgServiceNotAllowed})
	case kind == KindComeBack && !row.ServiceID.Valid && newReturn != nil:
		errs = append(errs, problem.FieldError{Field: "return_service_id", Message: problem.MsgGeneralNoReturn})
	}
	storedTemplateArchived := false
	if p.TemplateID != nil {
		if err := checkActive(ctx, &errs, "template_id", problem.MsgTemplateUnavailable, *p.TemplateID, queries.GetTemplateState); err != nil {
			return Rule{}, err
		}
		if *p.TemplateID != row.TemplateID {
			stored, err := queries.GetTemplateState(ctx, row.TemplateID)
			if err != nil {
				return Rule{}, fmt.Errorf("get stored template state: %w", err)
			}
			storedTemplateArchived = stored != nil
		}
	}
	storedEffective := row.ReturnServiceID
	if !storedEffective.Valid {
		storedEffective = row.ServiceID
	}
	newEffective := storedEffective
	if p.ReturnServiceID.IsSpecified() && kind == KindComeBack {
		newEffective = row.ServiceID
		if newReturn != nil {
			newEffective = uuid.NullUUID{UUID: *newReturn, Valid: true}
			if err := checkActive(ctx, &errs, "return_service_id", problem.MsgServiceUnavailable, *newReturn, queries.GetServiceState); err != nil {
				return Rule{}, err
			}
		}
	}
	if len(errs) > 0 {
		return Rule{}, problem.Validation{Errors: errs}
	}
	params := sqlc.UpdateRuleParams{
		ID: id, OffsetDays: row.OffsetDays, TemplateID: row.TemplateID,
		ReturnServiceID: row.ReturnServiceID, AppliesFrom: row.AppliesFrom, HandoverFrom: row.HandoverFrom,
	}
	asks := false
	if p.OffsetDays != nil {
		params.OffsetDays = offset
		asks = asks || offset != row.OffsetDays
	}
	if p.TemplateID != nil {
		params.TemplateID = *p.TemplateID
		asks = asks || (*p.TemplateID != row.TemplateID && storedTemplateArchived)
	}
	if p.ReturnServiceID.IsSpecified() && kind == KindComeBack {
		params.ReturnServiceID = uuid.NullUUID{}
		if newReturn != nil {
			params.ReturnServiceID = newEffective
		}
		asks = asks || newEffective != storedEffective
	}
	if p.Archived != nil && !*p.Archived && row.ArchivedAt != nil {
		asks = true
	}
	now := m.clock.Now()
	if asks {
		params.AppliesFrom = nil
		if !p.IncludeOverdue {
			params.AppliesFrom = &now
		}
	}
	params.ArchivedAt = archive.At(p.Archived, row.ArchivedAt, now)
	if kind == KindComeBack && row.ServiceID.Valid && (row.ArchivedAt == nil) != (params.ArchivedAt == nil) {
		// Archive or restore of a specific rule: it stops or starts sending only when its parts are
		// active once the change is made.
		parts, err := partsActive(ctx, queries, params.TemplateID, row.ServiceID.UUID, params.ReturnServiceID)
		if err != nil {
			return Rule{}, err
		}
		if parts {
			params.HandoverFrom = nil
			if params.ArchivedAt != nil && !p.IncludeOverdue {
				params.HandoverFrom = &now
			}
		}
	}
	if err := queries.UpdateRule(ctx, params); err != nil {
		return Rule{}, fmt.Errorf("update rule: %w", err)
	}
	row.OffsetDays, row.TemplateID, row.ReturnServiceID = params.OffsetDays, params.TemplateID, params.ReturnServiceID
	row.AppliesFrom, row.ArchivedAt, row.HandoverFrom = params.AppliesFrom, params.ArchivedAt, params.HandoverFrom
	return ruleFrom(row), nil
}

// partsActive reports whether a specific rule's template, service and effective return service
// (returnID, or the service when it is empty) are all active.
func partsActive(ctx context.Context, queries *sqlc.Queries, templateID, serviceID uuid.UUID, returnID uuid.NullUUID) (bool, error) {
	ids := []uuid.UUID{serviceID}
	if returnID.Valid {
		ids = append(ids, returnID.UUID)
	}
	tpl, err := queries.GetTemplateState(ctx, templateID)
	if err != nil {
		return false, fmt.Errorf("get template state: %w", err)
	}
	if tpl != nil {
		return false, nil
	}
	for _, id := range ids {
		svc, err := queries.GetServiceState(ctx, id)
		if err != nil {
			return false, fmt.Errorf("get service state: %w", err)
		}
		if svc != nil {
			return false, nil
		}
	}
	return true, nil
}

// HandoverScope names the template or the service whose archive or restore changes which specific
// rules can send. Exactly one is set.
type HandoverScope struct {
	TemplateID uuid.NullUUID
	ServiceID  uuid.NullUUID
}

// SetHandover answers the handover question for the template or service about to be archived: every
// specific come-back rule that can send under it gets HandoverFrom now, or nil with includeOverdue.
// The caller runs it before the archive, in the same transaction; on an archived template or
// service it changes nothing.
func (m *Module) SetHandover(ctx context.Context, q db.DBTX, scope HandoverScope, includeOverdue bool) error {
	if err := sqlc.New(q).SetHandover(ctx, sqlc.SetHandoverParams{
		TemplateID: scope.TemplateID, ServiceID: scope.ServiceID, HandoverFrom: m.answer(includeOverdue),
	}); err != nil {
		return fmt.Errorf("set handover: %w", err)
	}
	return nil
}

// RestartRules answers the include-overdue question for the template or service about to be
// restored: every specific come-back rule that cannot send only because of it gets AppliesFrom now
// (nil with includeOverdue), as a rule restore does, and loses its HandoverFrom. The caller runs it
// before the restore, in the same transaction; on an active template or service it changes nothing.
func (m *Module) RestartRules(ctx context.Context, q db.DBTX, scope HandoverScope, includeOverdue bool) error {
	if err := sqlc.New(q).RestartRules(ctx, sqlc.RestartRulesParams{
		TemplateID: scope.TemplateID, ServiceID: scope.ServiceID, AppliesFrom: m.answer(includeOverdue),
	}); err != nil {
		return fmt.Errorf("restart rules: %w", err)
	}
	return nil
}

// answer is the include-overdue answer as a time: now for "from today", nil for "include late".
func (m *Module) answer(includeOverdue bool) *time.Time {
	if includeOverdue {
		return nil
	}
	now := m.clock.Now()
	return &now
}

// offsetDays checks an offset against the range of the kind and converts it to the column type. The
// message is empty when the offset is valid.
func offsetDays(kind Kind, n int) (int32, string) {
	switch {
	case kind == KindComeBack && (n < 1 || n > maxOffsetDays):
		return 0, problem.MsgComeBackOffsetInvalid
	case n < 0 || n > maxOffsetDays:
		return 0, problem.MsgOffsetInvalid
	}
	return int32(n), ""
}

// checkActive adds a field error when the row id does not exist or is archived. getState is a
// GetTemplateState or GetServiceState query: it returns the archive time, or no rows.
func checkActive(
	ctx context.Context, errs *[]problem.FieldError, field, msg string, id uuid.UUID,
	getState func(context.Context, uuid.UUID) (*time.Time, error),
) error {
	archivedAt, err := getState(ctx, id)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		*errs = append(*errs, problem.FieldError{Field: field, Message: msg})
	case err != nil:
		return fmt.Errorf("get %s state: %w", field, err)
	case archivedAt != nil:
		*errs = append(*errs, problem.FieldError{Field: field, Message: msg})
	}
	return nil
}

func ruleFrom(r sqlc.ReminderRule) Rule {
	return Rule{
		ID: r.ID, Kind: Kind(r.Kind), ServiceID: r.ServiceID, ReturnServiceID: r.ReturnServiceID,
		OffsetDays: int(r.OffsetDays), TemplateID: r.TemplateID, AppliesFrom: r.AppliesFrom,
		Archived: r.ArchivedAt != nil, CreatedAt: r.CreatedAt, HandoverFrom: r.HandoverFrom,
	}
}
