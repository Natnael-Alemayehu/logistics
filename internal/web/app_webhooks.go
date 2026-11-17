package web

import (
	"net/http"
	"net/url"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/natnael-alemayehu/kiflet/internal/db"
	"github.com/natnael-alemayehu/kiflet/internal/service"
)

// Webhooks (FR-047), under Settings beside the API keys: a list of the
// company's endpoints, and a page for each with its secret and every
// message sent to it.

var webhookEventLabels = map[string]string{
	service.WebhookShipmentCreated: "A shipment is booked",
	service.WebhookStatusChanged:   "A shipment's status changes",
	service.WebhookDelivered:       "A shipment is delivered",
	service.WebhookIssueReported:   "A problem is reported, by the driver or the receiver",
}

type webhookEventOption struct{ Code, Label string }

func webhookEventOptions() []webhookEventOption {
	out := make([]webhookEventOption, 0, len(service.WebhookEvents))
	for _, e := range service.WebhookEvents {
		out = append(out, webhookEventOption{e, webhookEventLabels[e]})
	}
	return out
}

func (s *Server) webhooksView(w http.ResponseWriter, r *http.Request) (*View, error) {
	endpoints, err := s.svc.Webhooks.Endpoints(r.Context(), principal(r).TenantID)
	if err != nil {
		return nil, err
	}
	return s.view(w, r, "Webhooks", "settings", map[string]any{"Endpoints": endpoints, "Events": webhookEventOptions()}), nil
}

func (s *Server) webhooksPage(w http.ResponseWriter, r *http.Request) {
	v, err := s.webhooksView(w, r)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	v.Form = url.Values{"events": service.WebhookEvents}
	s.render(w, r, http.StatusOK, "app/webhooks", v)
}

func (s *Server) webhookCreate(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	e, err := s.svc.Webhooks.CreateEndpoint(r.Context(), principal(r), r.PostFormValue("url"), r.PostForm["events"], clientInfo(r))
	if err != nil {
		v, verr := s.webhooksView(w, r)
		if verr != nil {
			s.fail(w, r, verr)
			return
		}
		if !s.formError(w, r, "app/webhooks", v, err) {
			s.fail(w, r, err)
		}
		return
	}
	s.setFlash(w, "success", "Webhook added. Send it a test, and check the signature with the secret below.")
	http.Redirect(w, r, "/app/settings/webhooks/"+e.ID.String(), http.StatusSeeOther)
}

func (s *Server) webhookView(w http.ResponseWriter, r *http.Request) (*View, db.WebhookEndpoint, error) {
	id, err := pathID(r, "id")
	if err != nil {
		return nil, db.WebhookEndpoint{}, &service.Error{Kind: service.KindNotFound, Message: "That webhook doesn't exist."}
	}
	p := principal(r)
	e, err := s.svc.Webhooks.Endpoint(r.Context(), p.TenantID, id)
	if err != nil {
		return nil, e, err
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	page = max(page, 1)
	const per = 25
	rows, err := s.svc.Webhooks.Deliveries(r.Context(), p.TenantID, id, page, per+1)
	if err != nil {
		return nil, e, err
	}
	more := len(rows) > per
	if more {
		rows = rows[:per]
	}
	deliveries := make([]webhookDelivery, len(rows))
	for i, d := range rows {
		deliveries[i] = webhookDelivery{ListWebhookDeliveriesRow: d, Payload: service.WebhookPayload(d.Payload)}
	}
	v := s.view(w, r, "Webhook", "settings", map[string]any{
		"Endpoint": e, "Deliveries": deliveries, "Page": page, "More": more, "Events": webhookEventOptions(),
	})
	v.Form = url.Values{"url": {e.Url}, "events": e.Events}
	return v, e, nil
}

type webhookDelivery struct {
	db.ListWebhookDeliveriesRow
	Payload string
}

func (s *Server) webhookPage(w http.ResponseWriter, r *http.Request) {
	v, _, err := s.webhookView(w, r)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store") // the secret is on it
	s.render(w, r, http.StatusOK, "app/webhook", v)
}

func (s *Server) webhookUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.notFound(w, r)
		return
	}
	_ = r.ParseForm()
	_, err = s.svc.Webhooks.UpdateEndpoint(r.Context(), principal(r), id, r.PostFormValue("url"), r.PostForm["events"], clientInfo(r))
	if err != nil {
		v, _, verr := s.webhookView(w, r)
		if verr != nil {
			s.fail(w, r, verr)
			return
		}
		if !s.formError(w, r, "app/webhook", v, err) {
			s.fail(w, r, err)
		}
		return
	}
	s.setFlash(w, "success", "Webhook saved.")
	http.Redirect(w, r, "/app/settings/webhooks/"+id.String(), http.StatusSeeOther)
}

// webhookAction runs one of the endpoint page's buttons.
func (s *Server) webhookAction(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.notFound(w, r)
		return
	}
	p, ci, back := principal(r), clientInfo(r), "/app/settings/webhooks/"+id.String()
	var done string
	switch chi.URLParam(r, "action") {
	case "pause":
		err, done = s.svc.Webhooks.SetActive(r.Context(), p, id, false, ci), "Paused. Messages wait for it until it is resumed."
	case "resume":
		err, done = s.svc.Webhooks.SetActive(r.Context(), p, id, true, ci), "Resumed. Messages that waited are on their way."
	case "secret":
		err, done = s.svc.Webhooks.RollSecret(r.Context(), p, id, ci), "The webhook has a new secret. Messages are signed with it from now on."
	case "test":
		err, done = s.svc.Webhooks.SendTest(r.Context(), p, id), "A test is on its way. It shows below in a moment: reload to see how it went."
	case "delete":
		// Not back to its page, which is gone.
		if err := s.svc.Webhooks.DeleteEndpoint(r.Context(), p, id, ci); err != nil {
			s.actionResult(w, r, back, err, "")
			return
		}
		s.setFlash(w, "success", "Webhook removed, with its messages.")
		http.Redirect(w, r, "/app/settings/webhooks", http.StatusSeeOther)
		return
	default:
		s.notFound(w, r)
		return
	}
	s.actionResult(w, r, back, err, done)
}

func (s *Server) webhookResend(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.notFound(w, r)
		return
	}
	d, err := pathID(r, "delivery")
	if err != nil {
		s.notFound(w, r)
		return
	}
	err = s.svc.Webhooks.Resend(r.Context(), principal(r), d)
	s.actionResult(w, r, "/app/settings/webhooks/"+id.String(), err, "Sent again. Reload in a moment to see how it went.")
}
