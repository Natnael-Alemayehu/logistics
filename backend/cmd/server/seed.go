package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"math/rand/v2"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"github.com/natnael-alemayehu/kiflet/internal/app"
	"github.com/natnael-alemayehu/kiflet/internal/config"
	"github.com/natnael-alemayehu/kiflet/internal/db"
	"github.com/natnael-alemayehu/kiflet/internal/geo"
	"github.com/natnael-alemayehu/kiflet/internal/model"
	"github.com/natnael-alemayehu/kiflet/internal/service"
	"github.com/natnael-alemayehu/kiflet/pkg/hash"
)

// Demo credentials printed at the end of seeding.
const (
	demoPassword  = "kiflet-demo"
	adminPassword = "kiflet-admin"
	demoPIN       = "2580"
)

type demoRoute struct{ from, to string }

var (
	demoRoutes = []demoRoute{
		{"Addis Ababa", "Adama"}, {"Addis Ababa", "Dire Dawa"}, {"Addis Ababa", "Djibouti"},
		{"Addis Ababa", "Hawassa"}, {"Addis Ababa", "Bahir Dar"}, {"Addis Ababa", "Mekelle"},
		{"Adama", "Addis Ababa"}, {"Addis Ababa", "Jimma"}, {"Addis Ababa", "Gondar"}, {"Dire Dawa", "Jijiga"},
	}
	demoCustomers = []struct{ name, phone string }{
		{"Merkato Cement Traders", "0911223344"}, {"Hawassa Agro PLC", "0916554433"},
		{"Dire Dawa Textile Share Co.", "0915667788"}, {"Bahir Dar Coffee Union", "0918112233"},
		{"Mekelle Steel Works", "0914998877"}, {"Adama Flour Mill", "0922334455"},
		{"Jimma Coffee Export", "0917445566"}, {"Gondar Malt Factory", "0918665544"},
		{"Sheba Construction PLC", "0911887766"},
	}
	demoCargo = []struct {
		what string
		kg   float64
		etb  float64
	}{
		{"200 bags of cement", 10000, 180000}, {"Green coffee, 120 sacks", 7200, 1450000},
		{"Steel rebar, 12 mm", 18000, 1900000}, {"Wheat flour, 300 bags", 15000, 420000},
		{"Textile rolls", 6000, 980000}, {"Fertilizer (urea), 400 bags", 20000, 760000},
		{"Bottled water, 900 cases", 11000, 160000}, {"Ceramic floor tiles", 14000, 640000},
	}
	demoAddresses = map[string][]string{
		"Addis Ababa": {"Kality industrial zone, gate 3", "Merkato, near Anwar mosque", "Bole Lemi industrial park, shed 12", "Akaki, Mekanisa road warehouse"},
		"Adama":       {"Adama industrial zone, block B", "Franko area, Ras Hotel road"},
		"Dire Dawa":   {"Dire Dawa free trade zone, gate 1", "Sabiyan, near the railway station"},
		"Djibouti":    {"Port of Doraleh, container terminal", "PK12 logistics yard"},
		"Hawassa":     {"Hawassa industrial park, gate 2", "Piassa, next to the bus station"},
		"Bahir Dar":   {"Kebele 14 coffee warehouse", "Bahir Dar industrial village"},
		"Mekelle":     {"Mekelle industrial park", "Kedamay Weyane, steel depot"},
		"Jimma":       {"Jimma coffee auction centre", "Agaro road warehouse"},
		"Gondar":      {"Gondar malt factory, Azezo", "Piassa, Fasil road"},
		"Jijiga":      {"Jijiga main market depot", "Airport road warehouse"},
	}
	demoRecipients = []string{"Alemayehu Worku", "Meseret Tadesse", "Kebede Alemu", "Rahel Mekonnen", "Samuel Getachew", "Bethlehem Assefa", "Ahmed Mohammed", "Fikirte Desta"}
)

func seed(ctx context.Context, cfg *config.Config, pool *pgxpool.Pool, log zerolog.Logger) error {
	q := db.New(pool)
	var exists bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE email = 'demo@kiflet.et')`).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return errors.New("demo data is already loaded (demo@kiflet.et exists)")
	}
	a, err := app.New(cfg, pool, log, app.Options{})
	if err != nil {
		return err
	}
	svc := a.Services
	ci := service.ClientInfo{IP: "127.0.0.1", UserAgent: "seed"}
	rng := rand.New(rand.NewPCG(2026, 10))
	now := time.Now()

	// The platform operator's own account.
	plat, err := svc.Tenants.CreateCompany(ctx, service.SignupInput{
		Company: "Kiflet Platform", City: "Addis Ababa", AdminName: "Platform Admin",
		Email: "admin@kiflet.et", Phone: "0911000001", Password: adminPassword,
	}, ci)
	if err != nil {
		return fmt.Errorf("platform tenant: %w", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE users SET role = $2 WHERE id = $1`, plat.Principal.UserID, model.RolePlatformAdmin); err != nil {
		return err
	}
	// The platform's own account never pays: Enterprise limits, no period.
	if _, err := pool.Exec(ctx, `UPDATE tenants SET plan = 'enterprise', max_drivers = 500, sms_quota = 10000, sms_month_quota = 10000, trial_ends_at = NULL WHERE id = $1`, plat.Principal.TenantID); err != nil {
		return err
	}
	operator := plat.Principal
	operator.Role = model.RolePlatformAdmin

	// The demo company.
	res, err := svc.Tenants.CreateCompany(ctx, service.SignupInput{
		Company: "Abay Freight PLC", City: "Addis Ababa", AdminName: "Selam Tesfaye",
		Email: "demo@kiflet.et", Phone: "0911000002", Password: demoPassword,
	}, ci)
	if err != nil {
		return fmt.Errorf("demo tenant: %w", err)
	}
	admin := res.Principal
	tenantID := admin.TenantID
	if _, err := pool.Exec(ctx, `UPDATE tenants SET contact_phone = '0911000002', tin = '0012345678' WHERE id = $1`, tenantID); err != nil {
		return err
	}
	// The demo company pays for the Business plan the way a customer does:
	// it chooses the plan, and the platform operator records its bank
	// transfer. Its trial is over, so the paid month starts now.
	change, err := svc.Billing.ChoosePlan(ctx, admin, model.PlanBusiness, model.CycleMonthly, ci)
	if err != nil {
		return fmt.Errorf("demo plan: %w", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE tenants SET trial_ends_at = NOW() - INTERVAL '1 minute' WHERE id = $1`, tenantID); err != nil {
		return err
	}
	if err := svc.Billing.MarkPaid(ctx, operator, change.Invoice.ID, "CBE-DEMO-0001", "Demo data", ci); err != nil {
		return fmt.Errorf("demo payment: %w", err)
	}
	disp, err := svc.Users.CreateStaff(ctx, admin, service.PersonInput{
		FullName: "Dawit Bekele", Email: "dispatch@kiflet.et", Phone: "0911000003", Role: model.RoleDispatcher,
	}, ci)
	if err != nil {
		return fmt.Errorf("dispatcher: %w", err)
	}
	pw, err := hash.Password(demoPassword)
	if err != nil {
		return err
	}
	if err := q.UpdatePassword(ctx, db.UpdatePasswordParams{ID: disp.User.ID, PasswordHash: &pw, PasswordResetRequired: false}); err != nil {
		return err
	}
	// The demo dispatcher's address counts as confirmed, like the admin's.
	if _, err := pool.Exec(ctx, `UPDATE users SET email_verified_at = NOW() WHERE id = $1`, disp.User.ID); err != nil {
		return err
	}
	// Nobody needs the setup's emails: the demo addresses are confirmed, and
	// the first invoice is paid.
	if _, err := pool.Exec(ctx, `DELETE FROM emails WHERE tenant_id = $1 AND purpose IN ('email_verification', 'invoice', 'receipt')`, tenantID); err != nil {
		return err
	}

	vehicles := []service.VehicleInput{
		{PlateNumber: "3-A12345 AA", VehicleType: "heavy_truck", CapacityKg: 25000, Active: true},
		{PlateNumber: "3-B54321 AA", VehicleType: "trailer", CapacityKg: 40000, Active: true},
		{PlateNumber: "2-C77881 OR", VehicleType: "medium_truck", CapacityKg: 12000, Active: true},
		{PlateNumber: "3-A99012 AA", VehicleType: "light_truck", CapacityKg: 6000, Active: true},
		{PlateNumber: "3-B33210 AA", VehicleType: "refrigerated", CapacityKg: 8000, Active: true},
	}
	var vehicleIDs []pgtype.UUID
	for _, v := range vehicles {
		vh, err := svc.Vehicles.Create(ctx, admin, v, ci)
		if err != nil {
			return fmt.Errorf("vehicle %s: %w", v.PlateNumber, err)
		}
		vehicleIDs = append(vehicleIDs, vh.ID)
	}

	pinHash, err := hash.PIN(demoPIN)
	if err != nil {
		return err
	}
	driverInputs := []struct{ name, phone string }{
		{"Abebe Kebede", "0912345601"}, {"Tigist Haile", "0912345602"}, {"Mulugeta Alemu", "0912345603"},
		{"Hana Girma", "0912345604"}, {"Yonas Tadesse", "0912345605"},
	}
	var drivers []db.User
	for i, d := range driverInputs {
		acct, err := svc.Users.CreateDriver(ctx, admin, service.PersonInput{FullName: d.name, Phone: d.phone, VehicleID: vehicleIDs[i].String()}, ci)
		if err != nil {
			return fmt.Errorf("driver %s: %w", d.name, err)
		}
		if err := q.UpdatePIN(ctx, db.UpdatePINParams{ID: acct.User.ID, PinHash: &pinHash, PasswordResetRequired: false}); err != nil {
			return err
		}
		drivers = append(drivers, acct.User)
	}

	officeActor := admin.Actor(service.SourceDashboard, ci.IP, ci.UserAgent)
	driverActor := func(u db.User) service.Actor {
		return service.Actor{UserID: u.ID, TenantID: tenantID, Name: u.FullName, Role: model.RoleDriver, Source: service.SourceApp}
	}
	driverPrincipal := func(u db.User) *service.Principal {
		return &service.Principal{UserID: u.ID, TenantID: tenantID, Role: model.RoleDriver, Name: u.FullName, Phone: deref(u.Phone), TenantName: admin.TenantName}
	}
	pick := func(list []string) string { return list[rng.IntN(len(list))] }

	book := func(r demoRoute, driver int, priority string) (db.Shipment, error) {
		c := demoCustomers[rng.IntN(len(demoCustomers))]
		cargo := demoCargo[rng.IntN(len(demoCargo))]
		in := service.ShipmentInput{
			Priority: priority, OriginCity: r.from, OriginAddress: pick(demoAddresses[r.from]),
			DestinationCity: r.to, DestinationAddress: pick(demoAddresses[r.to]),
			CustomerName: c.name, CustomerPhone: c.phone, CargoDescription: cargo.what,
			CargoWeightKg: cargo.kg, CargoValueETB: cargo.etb,
		}
		if driver >= 0 {
			in.DriverID = drivers[driver].ID.String()
		}
		return svc.Shipments.Create(ctx, officeActor, in)
	}
	signature, err := demoSignature(rng)
	if err != nil {
		return err
	}

	// Six weeks of finished work, so the reports and ETAs have history.
	var history []pgtype.UUID
	for i := range 48 {
		r := demoRoutes[rng.IntN(len(demoRoutes))]
		dr := i % len(drivers)
		sh, err := book(r, dr, model.PriorityNormal)
		if err != nil {
			return fmt.Errorf("history shipment: %w", err)
		}
		a := driverActor(drivers[dr])
		for _, st := range []string{model.StatusInTransit, model.StatusArrived} {
			if _, err := svc.Shipments.ChangeStatus(ctx, a, sh.ID, service.StatusChange{Status: st, By: model.ActorDriver}); err != nil {
				return fmt.Errorf("history %s: %w", st, err)
			}
		}
		key, _, err := svc.Media.SavePOD(ctx, sh.ID.String(), signature, nil)
		if err != nil {
			return fmt.Errorf("signature: %w", err)
		}
		lat, lng := jitter(rng, *sh.DestinationLat, *sh.DestinationLng, 0.002)
		if _, err := svc.Shipments.RecordPOD(ctx, a, sh.ID, service.PODInput{
			RecipientName: demoRecipients[rng.IntN(len(demoRecipients))], SignatureKey: key, Lat: &lat, Lng: &lng,
		}); err != nil {
			return fmt.Errorf("history pod: %w", err)
		}

		// Move it back in time.
		transit := geo.EstimateTransit(*sh.OriginLat, *sh.OriginLng, *sh.DestinationLat, *sh.DestinationLng, nil)
		created := now.Add(-time.Duration(1+rng.IntN(44))*24*time.Hour - time.Duration(rng.IntN(10))*time.Hour)
		picked := created.Add(time.Duration(1+rng.IntN(4)) * time.Hour)
		eta := picked.Add(transit)
		factor := 0.72 + rng.Float64()*0.36 // about one in five runs late
		delivered := picked.Add(time.Duration(float64(transit) * factor))
		if err := backdate(ctx, pool, sh.ID, created, picked, eta, delivered, rng.IntN(3) > 0); err != nil {
			return err
		}
		history = append(history, sh.ID)
	}
	// Their texts were for the past; don't send them now.
	if _, err := pool.Exec(ctx, `DELETE FROM sms_messages WHERE shipment_id = ANY($1)`, history); err != nil {
		return err
	}

	// Today's board.
	type active struct {
		route    demoRoute
		driver   int
		priority string
		status   []string
		reason   string
		progress float64
	}
	board := []active{
		{demoRoute{"Addis Ababa", "Djibouti"}, 0, model.PriorityHigh, []string{model.StatusInTransit}, "", 0.45},
		{demoRoute{"Addis Ababa", "Hawassa"}, 1, model.PriorityNormal, []string{model.StatusInTransit}, "", 0.7},
		{demoRoute{"Addis Ababa", "Mekelle"}, 2, model.PriorityNormal, []string{model.StatusInTransit, model.StatusDelayed}, "road_conditions", 0.3},
		{demoRoute{"Adama", "Addis Ababa"}, 3, model.PriorityNormal, []string{model.StatusInTransit, model.StatusArrived}, "", 1},
		{demoRoute{"Addis Ababa", "Dire Dawa"}, 4, model.PriorityUrgent, []string{model.StatusInTransit, model.StatusIssue}, "breakdown", 0.55},
		{demoRoute{"Addis Ababa", "Bahir Dar"}, 0, model.PriorityNormal, nil, "", 0},
		{demoRoute{"Addis Ababa", "Jimma"}, 1, model.PriorityNormal, nil, "", 0},
		{demoRoute{"Addis Ababa", "Adama"}, -1, model.PriorityHigh, nil, "", 0},
		{demoRoute{"Addis Ababa", "Gondar"}, -1, model.PriorityNormal, nil, "", 0},
	}
	for _, b := range board {
		sh, err := book(b.route, b.driver, b.priority)
		if err != nil {
			return fmt.Errorf("active shipment: %w", err)
		}
		if b.driver < 0 {
			continue
		}
		u := drivers[b.driver]
		for _, st := range b.status {
			reason := ""
			if st == model.StatusDelayed || st == model.StatusIssue {
				reason = b.reason
			}
			if _, err := svc.Shipments.ChangeStatus(ctx, driverActor(u), sh.ID, service.StatusChange{Status: st, Reason: reason, By: model.ActorDriver}); err != nil {
				return fmt.Errorf("active %s: %w", st, err)
			}
		}
		if len(b.status) == 0 {
			continue
		}
		// A trail of GPS points from the pickup to where the truck is now.
		hours := geo.EstimateTransit(*sh.OriginLat, *sh.OriginLng, *sh.DestinationLat, *sh.DestinationLng, nil).Hours() * b.progress
		start := now.Add(-time.Duration((hours + 0.5) * float64(time.Hour)))
		if err := rewriteHistory(ctx, pool,
			stmt{`UPDATE shipments SET picked_up_at = $2::timestamptz, created_at = $2::timestamptz - interval '3 hours' WHERE id = $1`, []any{sh.ID, start}},
			stmt{`UPDATE shipment_events SET occurred_at = CASE
			    WHEN event_type IN ('created', 'assigned') THEN $2::timestamptz - interval '3 hours'
			    WHEN to_status = 'in_transit' THEN $2::timestamptz
			    ELSE occurred_at END
			  WHERE shipment_id = $1`, []any{sh.ID, start}},
			stmt{`UPDATE shipment_events SET created_at = occurred_at WHERE shipment_id = $1`, []any{sh.ID}},
		); err != nil {
			return err
		}
		var locs []service.SyncLocation
		steps := max(6, int(hours*4))
		for i := 0; i <= steps; i++ {
			f := b.progress * float64(i) / float64(steps)
			lat := *sh.OriginLat + (*sh.DestinationLat-*sh.OriginLat)*f
			lng := *sh.OriginLng + (*sh.DestinationLng-*sh.OriginLng)*f
			lat, lng = jitter(rng, lat, lng, 0.01*math.Sin(f*math.Pi))
			at := start.Add(time.Duration(float64(now.Sub(start)-3*time.Minute) * float64(i) / float64(steps)))
			speed := 45 + rng.Float64()*25
			locs = append(locs, service.SyncLocation{
				ClientID: fmt.Sprintf("seed-%s-%d", sh.ID.String(), i), ShipmentID: sh.ID.String(),
				Lat: lat, Lng: lng, Speed: &speed, RecordedAt: at,
			})
		}
		battery := 40 + rng.IntN(55)
		deviceTime := now
		if _, err := svc.Sync.Sync(ctx, driverPrincipal(u), service.SyncRequest{
			DeviceID: "seed-" + u.ID.String(), AppVersion: "1.0.0", DeviceTime: &deviceTime, Battery: &battery, Locations: locs,
		}, ci); err != nil {
			return fmt.Errorf("seed sync: %w", err)
		}
	}

	log.Info().Msg("demo data loaded")
	fmt.Printf(`
Demo data is loaded.

  Dashboard          %[1]s/login
    administrator    demo@kiflet.et      / %[2]s
    dispatcher       dispatch@kiflet.et  / %[2]s
    platform admin   admin@kiflet.et     / %[3]s

  Driver app         %[1]s/driver/
    drivers          0912345601 … 0912345605, PIN %[4]s

  Public tracking    %[1]s/track
`, cfg.PublicURL, demoPassword, adminPassword, demoPIN)
	return nil
}

// backdate moves a finished shipment and its history into the past.
func backdate(ctx context.Context, pool *pgxpool.Pool, id pgtype.UUID, created, picked, eta, delivered time.Time, confirmed bool) error {
	var confirmedAt *time.Time
	if confirmed {
		t := delivered.Add(90 * time.Minute)
		confirmedAt = &t
	}
	stmts := []stmt{
		{`UPDATE shipments SET created_at = $2::timestamptz, picked_up_at = $3, estimated_delivery = $4, actual_delivery = $5::timestamptz,
		     status_changed_at = $5::timestamptz, updated_at = $5::timestamptz, customer_confirmed_at = $6, eta_manual = false,
		     tracking_number = 'ET-' || to_char($2::timestamptz AT TIME ZONE 'Africa/Addis_Ababa', 'YYYYMMDD') || substr(tracking_number, 12)
		   WHERE id = $1`, []any{id, created, picked, eta, delivered, confirmedAt}},
		{`UPDATE shipment_events SET occurred_at = CASE
		     WHEN event_type IN ('created', 'assigned') THEN $2::timestamptz
		     WHEN to_status = 'in_transit' THEN $3::timestamptz
		     WHEN to_status = 'arrived' THEN $4::timestamptz - interval '25 minutes'
		     ELSE $4::timestamptz END
		   WHERE shipment_id = $1`, []any{id, created, picked, delivered}},
		{`UPDATE shipment_events SET created_at = occurred_at WHERE shipment_id = $1`, []any{id}},
		{`UPDATE proof_of_deliveries SET recorded_at = $2, synced_at = $2 WHERE shipment_id = $1`, []any{id, delivered}},
	}
	if err := rewriteHistory(ctx, pool, stmts...); err != nil {
		return fmt.Errorf("backdate: %w", err)
	}
	if confirmed {
		_, err := pool.Exec(ctx, `INSERT INTO shipment_events (tenant_id, shipment_id, event_type, actor_name, source, occurred_at, created_at)
			SELECT tenant_id, id, 'customer_confirmed', customer_name, 'customer', $2, $2 FROM shipments WHERE id = $1`, id, *confirmedAt)
		if err != nil {
			return fmt.Errorf("backdate confirmation: %w", err)
		}
	}
	return nil
}

type stmt struct {
	sql  string
	args []any
}

// rewriteHistory moves demo shipments' timelines into the past. The timeline
// is append-only (the append_only trigger), so the statements run in a
// transaction allowed to change it.
func rewriteHistory(ctx context.Context, pool *pgxpool.Pool, stmts ...stmt) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // a no-op after commit
	if _, err := tx.Exec(ctx, "SELECT set_config('kiflet.purge', 'on', true)"); err != nil {
		return err
	}
	for _, s := range stmts {
		if _, err := tx.Exec(ctx, s.sql, s.args...); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func jitter(rng *rand.Rand, lat, lng, deg float64) (float64, float64) {
	return lat + (rng.Float64()*2-1)*deg, lng + (rng.Float64()*2-1)*deg
}

// demoSignature draws a plausible scribble as a PNG data URL.
func demoSignature(rng *rand.Rand) (string, error) {
	const w, h = 360, 120
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = 0xff
	}
	ink := color.RGBA{20, 20, 30, 255}
	a, b := 2+rng.Float64()*2, 5+rng.Float64()*3
	for x := 20.0; x < w-20; x += 0.25 {
		y := h/2 + 28*math.Sin(x/w*math.Pi*a)*math.Cos(x/w*math.Pi*b)
		for dy := -1.0; dy <= 1; dy++ {
			img.Set(int(x), int(y+dy), ink)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

func deref[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}
	return *p
}
