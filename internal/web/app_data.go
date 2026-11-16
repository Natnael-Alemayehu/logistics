package web

import (
	"archive/zip"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/natnael-alemayehu/kiflet/internal/db"
	"github.com/natnael-alemayehu/kiflet/internal/model"
	"github.com/natnael-alemayehu/kiflet/internal/service"
	"github.com/natnael-alemayehu/kiflet/migrations"
)

// ---------------------------------------------------------------------------
// A company's data: its export, and removing a person's details (FR-041)
// ---------------------------------------------------------------------------

// exportData sends everything the company holds as one ZIP: a CSV for each
// kind of record, and its deliveries' signatures and photos (AC-041.5).
func (s *Server) exportData(w http.ResponseWriter, r *http.Request) {
	p := principal(r)
	ex, err := s.svc.Data.Export(r.Context(), p, clientInfo(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	name := fmt.Sprintf("%s-data-%s.zip", fileSlug(strings.ToLower(ex.Company.Name)), ex.At.In(s.cfg.Timezone).Format("2006-01-02"))
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, name))
	w.Header().Set("Cache-Control", "no-store")
	zw := zip.NewWriter(w)
	err = s.writeExport(r.Context(), zw, ex, p.Name)
	if err == nil {
		err = zw.Close()
	}
	if err != nil {
		// The download has started, so it can only be cut off: the browser
		// then reports it failed rather than saving a broken file.
		s.log.Error().Err(err).Msg("data export")
		panic(http.ErrAbortHandler)
	}
}

func (s *Server) writeExport(ctx context.Context, zw *zip.Writer, ex *service.Export, by string) error {
	loc := s.cfg.Timezone
	ts := func(t pgtype.Timestamptz) string {
		if !t.Valid {
			return ""
		}
		return t.Time.In(loc).Format("2006-01-02 15:04")
	}
	yesNo := func(b bool) string {
		if b {
			return "yes"
		}
		return "no"
	}
	coord := func(f *float64) string {
		if f == nil {
			return ""
		}
		return strconv.FormatFloat(*f, 'f', 6, 64)
	}
	// Each delivery's files sit under its shipment, numbered when a
	// shipment has more than one proof of delivery.
	type file struct{ key, name string }
	var media []file
	seen := map[string]int{}
	podFiles := make([][]string, len(ex.PODs))
	for i, pod := range ex.PODs {
		seen[pod.TrackingNumber]++
		dir := "deliveries/" + fileSlug(pod.TrackingNumber) + "/"
		if n := seen[pod.TrackingNumber]; n > 1 {
			dir += strconv.Itoa(n) + "-"
		}
		add := func(key, name string) {
			if key != "" {
				media = append(media, file{key, dir + name + path.Ext(key)})
				podFiles[i] = append(podFiles[i], dir+name+path.Ext(key))
			}
		}
		add(pod.SignatureKey, "signature")
		for j, k := range pod.PhotoKeys {
			add(k, fmt.Sprintf("photo-%d", j+1))
		}
	}

	readme := fmt.Sprintf(`%s: everything %s holds
Downloaded %s by %s.

shipments.csv    every shipment (%d)
events.csv       what happened to each shipment, in order (%d)
deliveries.csv   proofs of delivery (%d); their signatures and photos are in deliveries/
people.csv       drivers and office staff (%d)
vehicles.csv     vehicles (%d)

Times are in %s.

Shipments that finished more than %d days ago no longer hold their customer's
or recipient's details, signatures or photos: they are removed once the
keeping period is over. Those shipments say "Former customer". People who
left and whose details were removed are listed as "Former driver" or "Former
staff member".

A shipment's GPS route is kept for 90 days and is not in this file: download
it from the shipment's page while it is kept.
`, ex.Company.Name, s.cfg.AppName, ex.At.In(loc).Format("2 January 2006 at 15:04"), by,
		len(ex.Shipments), len(ex.Events), len(ex.PODs), len(ex.People), len(ex.Vehicles),
		loc.String(), int(s.svc.Data.RetainShipments().Hours()/24))
	if err := writeZipFile(zw, "README.txt", strings.NewReader(readme)); err != nil {
		return err
	}

	err := writeCSV(zw, "shipments.csv", []string{
		"Tracking number", "Status", "Reason", "Priority", "Booked", "Planned pickup", "Picked up", "Due", "Delivered",
		"From city", "From address", "To city", "To address", "Customer", "Customer phone", "Receiver", "Receiver phone",
		"Cargo", "Weight (kg)", "Value (ETB)", "Instructions", "Driver", "Vehicle",
		"Customer confirmed", "Customer feedback", "Details removed",
	}, len(ex.Shipments), func(i int) []string {
		sh := ex.Shipments[i]
		return []string{
			sh.TrackingNumber, model.StatusLabel(sh.Status), reasonText(sh.StatusReason), sh.Priority,
			ts(sh.CreatedAt), ts(sh.PlannedPickupAt), ts(sh.PickedUpAt), ts(sh.EstimatedDelivery), ts(sh.ActualDelivery),
			sh.OriginCity, csvSafe(sh.OriginAddress), sh.DestinationCity, csvSafe(sh.DestinationAddress),
			csvSafe(sh.CustomerName), csvSafe(sh.CustomerPhone), csvSafe(sh.ConsigneeName), csvSafe(sh.ConsigneePhone),
			csvSafe(sh.CargoDescription), numStr(sh.CargoWeightKg), numStr(sh.CargoValue), csvSafe(sh.SpecialInstructions),
			csvSafe(sh.DriverName), sh.VehiclePlate,
			ts(sh.CustomerConfirmedAt), csvSafe(sh.CustomerFeedback), ts(sh.AnonymisedAt),
		}
	})
	if err != nil {
		return err
	}
	err = writeCSV(zw, "events.csv", []string{
		"Tracking number", "Time", "Event", "From status", "To status", "Reason", "Note", "By", "Source", "Latitude", "Longitude",
	}, len(ex.Events), func(i int) []string {
		e := ex.Events[i]
		return []string{
			e.TrackingNumber, ts(e.OccurredAt), strings.ReplaceAll(e.EventType, "_", " "),
			statusText(e.FromStatus), statusText(e.ToStatus), reasonText(e.Reason), csvSafe(e.Note),
			csvSafe(e.ActorName), e.Source, coord(e.Latitude), coord(e.Longitude),
		}
	})
	if err != nil {
		return err
	}
	err = writeCSV(zw, "deliveries.csv", []string{
		"Tracking number", "Status", "Delivered", "Received by the office", "Recipient", "Recipient phone", "Driver",
		"Latitude", "Longitude", "At the drop-off", "Metres from the drop-off", "Reason away from the drop-off", "Notes", "Files",
	}, len(ex.PODs), func(i int) []string {
		pod := ex.PODs[i]
		lat, lng, near := "", "", ""
		if pod.Latitude != 0 || pod.Longitude != 0 {
			lat, lng = coord(&pod.Latitude), coord(&pod.Longitude)
		}
		if pod.LocationVerified != nil {
			near = yesNo(*pod.LocationVerified)
		}
		return []string{
			pod.TrackingNumber, pod.Status, ts(pod.RecordedAt), ts(pod.SyncedAt), csvSafe(pod.RecipientName), csvSafe(pod.RecipientPhone),
			csvSafe(pod.DriverName), lat, lng, near, numStr(pod.MismatchMeters), reasonText(pod.LocationReason),
			csvSafe(pod.DeliveryNotes), strings.Join(podFiles[i], " "),
		}
	})
	if err != nil {
		return err
	}
	err = writeCSV(zw, "people.csv", []string{
		"Name", "Role", "Phone", "Email", "Active", "Vehicle", "Added", "Last sign-in", "Details removed",
	}, len(ex.People), func(i int) []string {
		u := ex.People[i]
		return []string{
			csvSafe(u.FullName), model.RoleLabel(u.Role), u.Phone, csvSafe(u.Email), yesNo(u.IsActive), u.VehiclePlate,
			ts(u.CreatedAt), ts(u.LastLoginAt), ts(u.AnonymisedAt),
		}
	})
	if err != nil {
		return err
	}
	err = writeCSV(zw, "vehicles.csv", []string{"Plate", "Type", "Capacity (kg)", "Active", "Added"}, len(ex.Vehicles), func(i int) []string {
		v := ex.Vehicles[i]
		return []string{v.PlateNumber, service.VehicleTypeLabel(deref(v.VehicleType)), numStr(v.CapacityKg), yesNo(v.IsActive), ts(v.CreatedAt)}
	})
	if err != nil {
		return err
	}

	for _, f := range media {
		rc, _, err := s.svc.Media.Open(ctx, f.key)
		if err != nil {
			// A file lost from storage shouldn't cost the company the rest.
			s.log.Warn().Err(err).Str("key", f.key).Msg("export: delivery file missing")
			continue
		}
		err = writeZipFile(zw, f.name, rc)
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func writeZipFile(zw *zip.Writer, name string, body io.Reader) error {
	w, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, body)
	return err
}

func writeCSV(zw *zip.Writer, name string, header []string, n int, row func(int) []string) error {
	w, err := zw.Create(name)
	if err != nil {
		return err
	}
	// A byte-order mark, so Excel reads Amharic names as UTF-8.
	if _, err := w.Write([]byte("\ufeff")); err != nil {
		return err
	}
	cw := csv.NewWriter(w)
	_ = cw.Write(header)
	for i := range n {
		_ = cw.Write(row(i))
	}
	cw.Flush()
	return cw.Error()
}

func statusText(code string) string {
	if code == "" {
		return ""
	}
	return model.StatusLabel(code)
}

func reasonText(code string) string {
	if code == "" {
		return ""
	}
	return model.ReasonLabel(code)
}

// fileSlug makes a name safe for a file name: ASCII letters, digits and
// dashes.
func fileSlug(name string) string {
	var b strings.Builder
	dash := false
	for _, r := range name {
		if r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)) {
			b.WriteRune(r)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	out := strings.TrimRight(b.String(), "-")
	if out == "" {
		return "company"
	}
	return out
}

// personAnonymise removes what identifies a person who has left the company.
func (s *Server) personAnonymise(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.notFound(w, r)
		return
	}
	p := principal(r)
	u, err := s.svc.Users.Get(r.Context(), p.TenantID, id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	was, err := s.svc.Data.AnonymisePerson(r.Context(), p, id, clientInfo(r))
	s.actionResult(w, r, personPath(u), err, was+"'s name, phone, email and sign-in were removed. Their history stays.")
}

// ---------------------------------------------------------------------------
// Platform: deleting a company, the audit log and system health
// ---------------------------------------------------------------------------

func (s *Server) platformTenantDelete(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.notFound(w, r)
		return
	}
	name, err := s.svc.Data.DeleteCompany(r.Context(), principal(r), id, r.PostFormValue("confirm"), clientInfo(r))
	if err != nil {
		s.actionResult(w, r, "/app/platform", err, "")
		return
	}
	// The company's page, where the form was, is gone.
	s.setFlash(w, "success", name+" and everything it held were deleted. Its invoices are kept.")
	http.Redirect(w, r, "/app/platform", http.StatusSeeOther)
}

type platformAuditRow struct {
	auditRow
	Company     string
	CompanyLink string
}

type auditAction struct{ Code, Label string }

// platformAudit lets the platform admin search every company's audit log
// (AC-042.7).
func (s *Server) platformAudit(w http.ResponseWriter, r *http.Request) {
	p := principal(r)
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	page = max(page, 1)
	const per = 50
	f := service.AuditFilter{Action: q.Get("action"), IP: strings.TrimSpace(q.Get("ip"))}
	if id, err := parseID(q.Get("company")); err == nil {
		f.TenantID = id
	}
	loc := s.cfg.Timezone
	if t, err := time.ParseInLocation("2006-01-02", q.Get("from"), loc); err == nil {
		f.Since = t
	}
	if t, err := time.ParseInLocation("2006-01-02", q.Get("to"), loc); err == nil {
		f.Until = t.AddDate(0, 0, 1)
	}
	logs, err := s.svc.Audit.Platform(r.Context(), p, f, page, per+1)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	tenants, err := s.svc.Tenants.All(r.Context(), p)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	companies := map[pgtype.UUID]string{}
	for _, t := range tenants {
		companies[t.ID] = t.Name
	}
	more := len(logs) > per
	if more {
		logs = logs[:per]
	}
	rows := make([]platformAuditRow, 0, len(logs))
	for _, l := range logs {
		rows = append(rows, platformAuditEntry(l, companies))
	}
	actions := make([]auditAction, 0, len(auditActions))
	for code, label := range auditActions {
		actions = append(actions, auditAction{code, label})
	}
	slices.SortFunc(actions, func(a, b auditAction) int { return strings.Compare(a.Label, b.Label) })
	pageURL := func(n int) string {
		v := url.Values{}
		for k, vals := range q {
			if k != "page" && len(vals) > 0 && vals[0] != "" {
				v.Set(k, vals[0])
			}
		}
		if n > 1 {
			v.Set("page", strconv.Itoa(n))
		}
		if len(v) == 0 {
			return "/app/platform/audit"
		}
		return "/app/platform/audit?" + v.Encode()
	}
	data := map[string]any{"Rows": rows, "Page": page, "More": more, "Tenants": tenants, "Actions": actions}
	if page > 1 {
		data["Prev"] = pageURL(page - 1)
	}
	if more {
		data["Next"] = pageURL(page + 1)
	}
	v := s.view(w, r, "Audit log", "platform-audit", data)
	s.render(w, r, http.StatusOK, "app/platform_audit", v)
}

func platformAuditEntry(l db.ListPlatformAuditRow, companies map[pgtype.UUID]string) platformAuditRow {
	row := platformAuditRow{
		auditRow: auditRow{At: l.CreatedAt.Time, Who: l.UserName, Action: auditActions[l.Action], IP: deref(l.IpAddress)},
		Company:  l.TenantName,
	}
	if _, ok := companies[l.TenantID]; ok {
		row.CompanyLink = tenantPath(l.TenantID)
	}
	if row.Who == "" {
		row.Who = "System"
	}
	if row.Action == "" {
		row.Action = strings.ReplaceAll(l.Action, "_", " ")
	}
	if l.EntityType == "tenant" && l.EntityID != l.TenantID {
		// The platform acting on a company: name the company, or the one
		// that was deleted.
		if n, ok := companies[l.EntityID]; ok {
			row.Entity, row.Link = n, tenantPath(l.EntityID)
		} else if n := auditName(l.NewValue); n != "" {
			row.Entity = n
		}
	}
	row.Changes = auditDiff(l.OldValue, l.NewValue)
	return row
}

// auditName is the "name" an audit row recorded, such as a deleted
// company's.
func auditName(raw []byte) string {
	for _, c := range auditDiff(nil, raw) {
		if c.Field == "name" {
			return c.To
		}
	}
	return ""
}

// latestMigration is the newest schema migration this build carries.
func latestMigration() int64 {
	entries, _ := fs.ReadDir(migrations.FS, ".")
	var latest int64
	for _, e := range entries {
		v, err := strconv.ParseInt(strings.SplitN(e.Name(), "_", 2)[0], 10, 64)
		if err == nil && v > latest {
			latest = v
		}
	}
	return latest
}

type provider struct {
	What, Name string
	// Mock is a stand-in in this server; Sandbox a real adapter talking to
	// the provider sandbox.
	Mock, Sandbox bool
}

// platformHealth shows how the system is running (AC-049.6).
func (s *Server) platformHealth(w http.ResponseWriter, r *http.Request) {
	h, err := s.svc.Health.Check(r.Context(), principal(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	sandbox := s.cfg.SandboxURL != ""
	data := map[string]any{
		"Health": h, "Latest": latestMigration(), "Version": s.version, "Uptime": time.Since(s.started),
		"Storage": s.cfg.StorageType,
		"Providers": []provider{
			{"Text messages", s.cfg.SMSProvider, s.mockSMS, !s.mockSMS && sandbox},
			{"Email", s.cfg.MailProvider, s.mockMail, false},
			{"Payments", s.cfg.PaymentProvider, s.mockPay, !s.mockPay && sandbox && s.cfg.PaymentProvider != "none"},
		},
	}
	if s.hub != nil {
		data["Sockets"] = s.hub.ClientCount()
	}
	if s.cfg.StorageType == "local" {
		if free, total, err := diskSpace(s.cfg.StoragePath); err == nil {
			data["DiskFree"], data["DiskTotal"] = free, total
			data["DiskLow"] = free < total/10 || free < 1<<30
		}
	} else {
		data["Bucket"] = s.cfg.S3Bucket
	}
	v := s.view(w, r, "System health", "platform-health", data)
	s.render(w, r, http.StatusOK, "app/platform_health", v)
}
