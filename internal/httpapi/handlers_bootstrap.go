package httpapi

import (
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/jnaranj09/open-customer/internal/httpapi/gen"
	"github.com/jnaranj09/open-customer/internal/locale"
	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

// maxBootstrapRangeDays is the longest GET /bootstrap window: to is at most this many days after
// from.
const maxBootstrapRangeDays = 200

// GetBootstrap answers everything the app caches. The range is checked first, then every part is
// read in one snapshot, in the order of the design doc (item 33). server_time is the same now
// DueList gets.
func (s *Server) GetBootstrap(w http.ResponseWriter, r *http.Request, params gen.GetBootstrapParams) {
	// A Date is midnight UTC, so the difference is a whole number of calendar days.
	span := params.To.Sub(params.From.Time)
	if span < 0 || span > maxBootstrapRangeDays*24*time.Hour {
		writeError(w, r, problem.Field("to", problem.MsgBootstrapRangeInvalid))
		return
	}
	ctx := r.Context()
	now := s.clock.Now()
	out := gen.Bootstrap{ServerTime: now, BackupRed: s.status.BackupRed()}
	if err := s.snapshot(r, func(tx pgx.Tx) error {
		cfg, err := s.settings.Get(ctx, tx)
		if err != nil {
			return fmt.Errorf("bootstrap settings: %w", err)
		}
		out.Settings = toGenSettings(cfg)
		tz, err := time.LoadLocation(cfg.Timezone)
		if err != nil {
			return fmt.Errorf("bootstrap time zone: %w", err)
		}
		lang, err := locale.For(cfg.Locale)
		if err != nil {
			return fmt.Errorf("bootstrap locale: %w", err)
		}
		attrs, err := s.attributes.List(ctx, tx)
		if err != nil {
			return fmt.Errorf("bootstrap attributes: %w", err)
		}
		out.Attributes = make([]gen.Attribute, 0, len(attrs))
		for _, a := range attrs {
			out.Attributes = append(out.Attributes, toGenAttribute(a))
		}
		svcs, err := s.services.List(ctx, tx)
		if err != nil {
			return fmt.Errorf("bootstrap services: %w", err)
		}
		out.Services = make([]gen.Service, 0, len(svcs))
		for _, v := range svcs {
			out.Services = append(out.Services, toGenService(v))
		}
		tmpls, err := s.templates.List(ctx, tx)
		if err != nil {
			return fmt.Errorf("bootstrap templates: %w", err)
		}
		out.Templates = make([]gen.Template, 0, len(tmpls))
		for _, t := range tmpls {
			out.Templates = append(out.Templates, toGenTemplate(t))
		}
		rules, err := s.reminders.ListRules(ctx, tx)
		if err != nil {
			return fmt.Errorf("bootstrap rules: %w", err)
		}
		out.Rules = make([]gen.Rule, 0, len(rules))
		for _, v := range rules {
			out.Rules = append(out.Rules, toGenRule(v))
		}
		cs, err := s.clients.List(ctx, tx, false)
		if err != nil {
			return fmt.Errorf("bootstrap clients: %w", err)
		}
		out.Clients = make([]gen.Client, 0, len(cs))
		for _, c := range cs {
			out.Clients = append(out.Clients, toGenClient(c))
		}
		appts, err := s.appointments.ListRange(ctx, tx, params.From.Time, params.To.Time, tz)
		if err != nil {
			return fmt.Errorf("bootstrap appointments: %w", err)
		}
		out.Appointments = toGenAppointments(appts)
		due, err := s.reminders.DueList(ctx, tx, now)
		if err != nil {
			return fmt.Errorf("bootstrap due list: %w", err)
		}
		out.Due = make([]gen.DueItem, 0, len(due))
		for _, it := range due {
			out.Due = append(out.Due, toGenDueItem(it))
		}
		vars, err := s.templates.Vars(ctx, tx, lang)
		if err != nil {
			return fmt.Errorf("bootstrap template variables: %w", err)
		}
		out.TemplateVars = make([]gen.TemplateVar, 0, len(vars))
		for _, v := range vars {
			out.TemplateVars = append(out.TemplateVars, gen.TemplateVar{Ns: v.NS, Key: v.Key, Label: v.Label})
		}
		return nil
	}); err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
