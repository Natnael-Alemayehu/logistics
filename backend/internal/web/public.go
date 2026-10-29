package web

import (
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/natnael-alemayehu/logistics/internal/geo"
	"github.com/natnael-alemayehu/logistics/internal/model"
	"github.com/natnael-alemayehu/logistics/internal/service"
)

func (s *Server) landing(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		s.notFound(w, r)
		return
	}
	s.render(w, r, http.StatusOK, "public/landing", s.view(w, r, "", "", map[string]any{"Plans": model.Plans[1:]}))
}

// ---------------------------------------------------------------------------
// Sign in, sign up, sign out
// ---------------------------------------------------------------------------

func safeNext(next string) string {
	if localPath(next) && strings.HasPrefix(next, "/app") {
		return next
	}
	return "/app"
}

func (s *Server) loginPage(w http.ResponseWriter, r *http.Request) {
	if principal(r) != nil {
		http.Redirect(w, r, "/app", http.StatusSeeOther)
		return
	}
	v := s.view(w, r, "Sign in", "", map[string]string{"Next": safeNext(r.URL.Query().Get("next"))})
	s.render(w, r, http.StatusOK, "public/login", v)
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	next := safeNext(r.PostFormValue("next"))
	res, err := s.svc.Auth.StaffLogin(r.Context(), r.PostFormValue("identifier"), r.PostFormValue("password"),
		r.PostFormValue("remember") != "", clientInfo(r))
	if err != nil {
		v := s.view(w, r, "Sign in", "", map[string]string{"Next": next})
		if !s.formError(w, r, "public/login", v, err) {
			s.fail(w, r, err)
		}
		return
	}
	var persist time.Time
	if r.PostFormValue("remember") != "" {
		persist = res.ExpiresAt
	}
	s.setSessionCookie(w, res.Token, persist)
	if res.Principal.IsPlatformAdmin() && next == "/app" {
		next = "/app/platform"
	}
	http.Redirect(w, r, next, http.StatusSeeOther)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if p := principal(r); p != nil {
		_ = s.svc.Auth.Logout(r.Context(), p)
	}
	s.clearSessionCookie(w)
	s.setFlash(w, "info", "You're signed out.")
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (s *Server) signupPage(w http.ResponseWriter, r *http.Request) {
	v := s.view(w, r, "Start your free trial", "", map[string]any{"Towns": geo.Places()})
	s.render(w, r, http.StatusOK, "public/signup", v)
}

func (s *Server) signup(w http.ResponseWriter, r *http.Request) {
	res, err := s.svc.Tenants.Signup(r.Context(), service.SignupInput{
		Company: r.PostFormValue("company"), City: r.PostFormValue("city"),
		AdminName: r.PostFormValue("name"), Email: r.PostFormValue("email"),
		Phone: r.PostFormValue("phone"), Password: r.PostFormValue("password"),
	}, clientInfo(r))
	if err != nil {
		v := s.view(w, r, "Start your free trial", "", map[string]any{"Towns": geo.Places()})
		if !s.formError(w, r, "public/signup", v, err) {
			s.fail(w, r, err)
		}
		return
	}
	s.setSessionCookie(w, res.Token, time.Time{})
	s.setFlash(w, "success", "Welcome! Add a vehicle and your first driver, then book a shipment.")
	http.Redirect(w, r, "/app", http.StatusSeeOther)
}

// passwordPage is the change-password form, which is also where someone
// signed in with an issued password must go first.
func (s *Server) passwordPage(w http.ResponseWriter, r *http.Request) {
	v := s.view(w, r, "Choose a password", "account", nil)
	s.render(w, r, http.StatusOK, "app/password", v)
}

func (s *Server) passwordChange(w http.ResponseWriter, r *http.Request) {
	p := principal(r)
	if r.PostFormValue("new_password") != r.PostFormValue("confirm_password") {
		v := s.view(w, r, "Choose a password", "account", nil)
		_ = s.formError(w, r, "app/password", v, &service.Error{Kind: service.KindInvalid, Field: "confirm_password", Message: "The passwords don't match."})
		return
	}
	err := s.svc.Auth.ChangePassword(r.Context(), p, r.PostFormValue("current_password"), r.PostFormValue("new_password"), clientInfo(r))
	if err != nil {
		v := s.view(w, r, "Choose a password", "account", nil)
		if !s.formError(w, r, "app/password", v, err) {
			s.fail(w, r, err)
		}
		return
	}
	s.setFlash(w, "success", "Password changed. Other devices have been signed out.")
	http.Redirect(w, r, "/app", http.StatusSeeOther)
}

// ---------------------------------------------------------------------------
// Public tracking (FR-029)
// ---------------------------------------------------------------------------

func (s *Server) trackForm(w http.ResponseWriter, r *http.Request) {
	if n := strings.TrimSpace(r.URL.Query().Get("n")); n != "" {
		http.Redirect(w, r, "/track/"+url.PathEscape(strings.ToUpper(n)), http.StatusSeeOther)
		return
	}
	s.render(w, r, http.StatusOK, "public/track_form", s.view(w, r, "Track a shipment", "", nil))
}

func (s *Server) trackSubmit(w http.ResponseWriter, r *http.Request) {
	n := strings.ToUpper(strings.Join(strings.Fields(r.PostFormValue("number")), ""))
	if n == "" {
		v := s.view(w, r, "Track a shipment", "", nil)
		_ = s.formError(w, r, "public/track_form", v, &service.Error{Kind: service.KindInvalid, Field: "number", Message: "Enter your tracking number."})
		return
	}
	http.Redirect(w, r, "/track/"+url.PathEscape(n), http.StatusSeeOther)
}

func (s *Server) trackPage(w http.ResponseWriter, r *http.Request) {
	t, err := s.svc.Shipments.Track(r.Context(), chi.URLParam(r, "number"))
	if service.IsNotFound(err) {
		v := s.view(w, r, "Track a shipment", "", nil)
		v.Form = url.Values{"number": {chi.URLParam(r, "number")}}
		v.Errors["number"] = "We couldn't find that tracking number. Check it and try again."
		s.render(w, r, http.StatusNotFound, "public/track_form", v)
		return
	}
	if err != nil {
		s.fail(w, r, err)
		return
	}
	v := s.view(w, r, t.TrackingNumber, "", t)
	s.render(w, r, http.StatusOK, "public/track", v)
}

const lookupCookie = "lookup"

func (s *Server) trackPhonePage(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, http.StatusOK, "public/track_phone", s.view(w, r, "Find my shipments", "", map[string]any{"Step": "phone"}))
}

func (s *Server) trackPhoneRequest(w http.ResponseWriter, r *http.Request) {
	p, err := s.svc.Shipments.RequestLookupCode(r.Context(), r.PostFormValue("phone"))
	if err != nil {
		v := s.view(w, r, "Find my shipments", "", map[string]any{"Step": "phone"})
		if !s.formError(w, r, "public/track_phone", v, err) {
			s.fail(w, r, err)
		}
		return
	}
	v := s.view(w, r, "Find my shipments", "", map[string]any{"Step": "code", "Phone": p})
	s.render(w, r, http.StatusOK, "public/track_phone", v)
}

func (s *Server) trackPhoneVerify(w http.ResponseWriter, r *http.Request) {
	token, err := s.svc.Shipments.VerifyLookupCode(r.Context(), r.PostFormValue("phone"), r.PostFormValue("code"))
	if err != nil {
		v := s.view(w, r, "Find my shipments", "", map[string]any{"Step": "code", "Phone": r.PostFormValue("phone")})
		if !s.formError(w, r, "public/track_phone", v, err) {
			s.fail(w, r, err)
		}
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: lookupCookie, Value: token, Path: "/track", MaxAge: 1800,
		HttpOnly: true, Secure: s.cfg.SecureCookies(), SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, "/track/mine", http.StatusSeeOther)
}

func (s *Server) trackMine(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie(lookupCookie)
	if err != nil {
		http.Redirect(w, r, "/track/phone", http.StatusSeeOther)
		return
	}
	p, ok := s.svc.Shipments.LookupPhone(c.Value)
	if !ok {
		http.Redirect(w, r, "/track/phone", http.StatusSeeOther)
		return
	}
	rows, err := s.svc.Shipments.ShipmentsForPhone(r.Context(), p)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	v := s.view(w, r, "Your shipments", "", map[string]any{"Phone": p, "Shipments": rows})
	s.render(w, r, http.StatusOK, "public/track_mine", v)
}

// ---------------------------------------------------------------------------
// Delivery confirmation (/c/{token})
// ---------------------------------------------------------------------------

func (s *Server) confirmPage(w http.ResponseWriter, r *http.Request) {
	cv, err := s.svc.Shipments.ByConfirmToken(r.Context(), chi.URLParam(r, "token"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	v := s.view(w, r, "Your delivery", "", map[string]any{"C": cv, "Token": chi.URLParam(r, "token")})
	s.render(w, r, http.StatusOK, "public/confirm", v)
}

func (s *Server) confirmSubmit(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	ok := r.PostFormValue("answer") == "ok"
	if err := s.svc.Shipments.CustomerRespond(r.Context(), token, ok, r.PostFormValue("feedback")); err != nil {
		if de, isDomain := service.AsError(err); isDomain && de.Kind == service.KindInvalid {
			s.setFlash(w, "error", de.Message)
			http.Redirect(w, r, "/c/"+token+"#problem", http.StatusSeeOther)
			return
		}
		s.fail(w, r, err)
		return
	}
	if ok {
		s.setFlash(w, "success", "Thank you for confirming.")
	} else {
		s.setFlash(w, "success", "Thank you. The company has been told and will contact you.")
	}
	http.Redirect(w, r, "/c/"+token, http.StatusSeeOther)
}

// confirmMedia serves the POD images to the holder of the confirmation link.
// n is 0 for the signature and 1.. for photos.
func (s *Server) confirmMedia(w http.ResponseWriter, r *http.Request) {
	cv, err := s.svc.Shipments.ByConfirmToken(r.Context(), chi.URLParam(r, "token"))
	if err != nil || cv.POD == nil {
		s.notFound(w, r)
		return
	}
	s.serveMedia(w, r, podMediaKey(cv.POD.SignatureUrl, cv.POD.PhotoUrls, chi.URLParam(r, "n")))
}

func podMediaKey(signature *string, photos []string, n string) string {
	i, err := strconv.Atoi(n)
	if err != nil || i < 0 {
		return ""
	}
	if i == 0 {
		if signature == nil {
			return ""
		}
		return *signature
	}
	if i > len(photos) {
		return ""
	}
	return photos[i-1]
}

func (s *Server) serveMedia(w http.ResponseWriter, r *http.Request, key string) {
	if key == "" {
		s.notFound(w, r)
		return
	}
	rc, ct, err := s.svc.Media.Open(r.Context(), key)
	if err != nil {
		if service.IsNotFound(err) {
			s.notFound(w, r)
			return
		}
		s.fail(w, r, err)
		return
	}
	defer rc.Close()
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "private, max-age=86400")
	// An uploaded SVG signature is drawn, never executed.
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
	_, _ = io.Copy(w, rc)
}
