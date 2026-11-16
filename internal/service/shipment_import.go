package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/natnael-alemayehu/kiflet/internal/db"
)

// Booking many shipments from a spreadsheet (FR-013 AC-013.1). The file is
// read and every row checked before anything is booked, and nothing is
// booked unless every row passes: half a file booked is a file that can't
// be sent again without booking that half twice.

// MaxImportRows is the most shipments one file may book.
const MaxImportRows = 200

// MaxImportBytes bounds the file.
const MaxImportBytes = 512 << 10

// importColumn is a column the import reads.
type importColumn struct {
	// Name is the heading as the template and the export spell it.
	Name     string
	Required bool
	Help     string
	// aliases are other spellings, compared with letters and digits only.
	aliases []string
	set     func(in *ShipmentInput, v string, loc *time.Location) error
}

func words(set func(in *ShipmentInput, v string)) func(*ShipmentInput, string, *time.Location) error {
	return func(in *ShipmentInput, v string, _ *time.Location) error { set(in, v); return nil }
}

func amount(what string, set func(in *ShipmentInput, f float64)) func(*ShipmentInput, string, *time.Location) error {
	return func(in *ShipmentInput, v string, _ *time.Location) error {
		if v == "" {
			return nil
		}
		f, err := strconv.ParseFloat(strings.ReplaceAll(v, ",", ""), 64)
		if err != nil {
			return fmt.Errorf("%s %q is not a number", what, v)
		}
		set(in, f)
		return nil
	}
}

// importTimeLayouts are the ways a spreadsheet writes a moment.
var importTimeLayouts = []string{"2006-01-02 15:04", "2006-01-02T15:04", "2006-01-02 15:04:05", "2006-01-02T15:04:05", "02/01/2006 15:04", "2006-01-02"}

func moment(what string, set func(in *ShipmentInput, t time.Time)) func(*ShipmentInput, string, *time.Location) error {
	return func(in *ShipmentInput, v string, loc *time.Location) error {
		if v == "" {
			return nil
		}
		for _, layout := range importTimeLayouts {
			if t, err := time.ParseInLocation(layout, v, loc); err == nil {
				set(in, t)
				return nil
			}
		}
		return fmt.Errorf("%s %q is not a date and time: write it as 2026-11-20 08:00", what, v)
	}
}

// ImportColumns are the columns of an import file, in the template's order.
// The first four are required; any other column in a file is ignored, so
// the shipment list's own export can be edited and sent back.
var ImportColumns = []importColumn{
	{Name: "Origin", Required: true, Help: "A town from the list, as on the booking form", aliases: []string{"origincity", "from", "fromcity"},
		set: words(func(in *ShipmentInput, v string) { in.OriginCity = v })},
	{Name: "Origin address", Help: "Where in the town to collect", aliases: []string{"fromaddress"},
		set: words(func(in *ShipmentInput, v string) { in.OriginAddress = v })},
	{Name: "Destination", Required: true, Help: "A town from the list", aliases: []string{"destinationcity", "to", "tocity"},
		set: words(func(in *ShipmentInput, v string) { in.DestinationCity = v })},
	{Name: "Destination address", Help: "Where in the town to deliver", aliases: []string{"toaddress"},
		set: words(func(in *ShipmentInput, v string) { in.DestinationAddress = v })},
	{Name: "Customer", Required: true, Help: "Who booked the shipment", aliases: []string{"customername"},
		set: words(func(in *ShipmentInput, v string) { in.CustomerName = v })},
	{Name: "Customer phone", Required: true, Help: "09… or +251…",
		set: words(func(in *ShipmentInput, v string) { in.CustomerPhone = v })},
	{Name: "Receiver", Help: "Who takes delivery, if not the customer", aliases: []string{"receivername", "consignee", "consigneename"},
		set: words(func(in *ShipmentInput, v string) { in.ConsigneeName = v })},
	{Name: "Receiver phone", Help: "Needed with a receiver", aliases: []string{"consigneephone"},
		set: words(func(in *ShipmentInput, v string) { in.ConsigneePhone = v })},
	{Name: "Cargo", Help: "What is carried", aliases: []string{"cargodescription"},
		set: words(func(in *ShipmentInput, v string) { in.CargoDescription = v })},
	{Name: "Weight (kg)", aliases: []string{"weight", "cargoweight"},
		set: amount("The weight", func(in *ShipmentInput, f float64) { in.CargoWeightKg = f })},
	{Name: "Value (ETB)", aliases: []string{"value", "cargovalue"},
		set: amount("The value", func(in *ShipmentInput, f float64) { in.CargoValueETB = f })},
	{Name: "Priority", Help: "normal, high or urgent; normal when empty",
		set: words(func(in *ShipmentInput, v string) { in.Priority = strings.ToLower(v) })},
	{Name: "Pickup", Help: "When the load is collected, as 2026-11-20 08:00", aliases: []string{"plannedpickup"},
		set: moment("The pickup time", func(in *ShipmentInput, t time.Time) { in.PlannedPickup = &t })},
	{Name: "Due", Help: "When it must arrive; estimated from the route when empty",
		set: moment("The due time", func(in *ShipmentInput, t time.Time) { in.EstimatedDelivery = &t })},
	{Name: "Instructions", Help: "For the driver", aliases: []string{"specialinstructions"},
		set: words(func(in *ShipmentInput, v string) { in.SpecialInstructions = v })},
}

// heading reduces a column heading to its letters and digits, lower case,
// so "Weight (kg)", "weight_kg" and "WEIGHT KG" are one.
func heading(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ImportTemplate is a file to start from: the headings and one example row.
func ImportTemplate() []byte {
	var b bytes.Buffer
	b.WriteString("\xEF\xBB\xBF") // so Excel reads UTF-8
	w := csv.NewWriter(&b)
	names := make([]string, len(ImportColumns))
	for i, c := range ImportColumns {
		names[i] = c.Name
	}
	_ = w.Write(names)
	_ = w.Write([]string{"Addis Ababa", "Kality depot, gate 2", "Hawassa", "Lakeside store", "Abyssinia Traders", "0911223344",
		"Lakeside Hardware", "0913220011", "Cement, 200 bags", "10000", "180000", "normal", "2026-11-20 08:00", "", "Call before arriving"})
	w.Flush()
	return b.Bytes()
}

// ImportRow is one shipment of an import file.
type ImportRow struct {
	// Line is its line in the file, counting the headings as line 1.
	Line  int
	Input ShipmentInput
	// Error is why the row can't be booked, or empty.
	Error string
}

// Import is an import file as read: its rows, checked.
type Import struct {
	Rows []ImportRow
	// Ignored are the file's columns the import doesn't read.
	Ignored []string
	// Bad counts the rows with an Error.
	Bad int
}

// ReadImport reads an import file and checks every row as the booking form
// would. Its error is about the file as a whole; a row's own is in the row.
func (s *ShipmentService) ReadImport(file []byte) (*Import, error) {
	if len(file) > MaxImportBytes {
		return nil, invalid("file", fmt.Sprintf("That file is too large. An import holds at most %d shipments.", MaxImportRows))
	}
	file = bytes.TrimPrefix(file, []byte("\xEF\xBB\xBF"))
	first, _, _ := bytes.Cut(file, []byte("\n"))
	r := csv.NewReader(bytes.NewReader(file))
	// Excel writes semicolons where the comma is the decimal mark.
	if bytes.Count(first, []byte(";")) > bytes.Count(first, []byte(",")) {
		r.Comma = ';'
	}
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	r.TrimLeadingSpace = true

	head, err := r.Read()
	if errors.Is(err, io.EOF) {
		return nil, invalid("file", "That file is empty.")
	}
	if err != nil {
		return nil, invalid("file", "That file isn't CSV. In a spreadsheet, save it as CSV (comma delimited) and choose it again.")
	}
	out := &Import{}
	cols := make([]*importColumn, len(head))
	seen := map[*importColumn]bool{}
	for i, h := range head {
		key := heading(h)
		if key == "" {
			continue
		}
		for j := range ImportColumns {
			c := &ImportColumns[j]
			match := heading(c.Name) == key
			for _, a := range c.aliases {
				match = match || a == key
			}
			if match && !seen[c] {
				cols[i], seen[c] = c, true
				break
			}
		}
		if cols[i] == nil {
			out.Ignored = append(out.Ignored, strings.TrimSpace(h))
		}
	}
	var missing []string
	for j := range ImportColumns {
		if c := &ImportColumns[j]; c.Required && !seen[c] {
			missing = append(missing, c.Name)
		}
	}
	if len(missing) > 0 {
		return nil, invalid("file", "The first line must name the columns, and these are missing: "+strings.Join(missing, ", ")+".")
	}

	now := s.now()
	for line := 2; ; line++ {
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, invalid("file", fmt.Sprintf("Line %d can't be read as CSV: check its quotation marks.", line))
		}
		row := ImportRow{Line: line}
		empty := true
		for i, v := range rec {
			v = strings.TrimSpace(v)
			if v == "" || i >= len(cols) || cols[i] == nil {
				continue
			}
			empty = false
			if err := cols[i].set(&row.Input, v, s.cfg.Location); err != nil && row.Error == "" {
				row.Error = upperFirst(err.Error()) + "."
			}
		}
		if empty {
			continue
		}
		if row.Error == "" {
			if _, err := validateShipment(row.Input, now); err != nil {
				row.Error = err.Error()
			}
		}
		if row.Error != "" {
			out.Bad++
		}
		out.Rows = append(out.Rows, row)
		if len(out.Rows) > MaxImportRows {
			return nil, invalid("file", fmt.Sprintf("An import holds at most %d shipments. Split the file and import each part.", MaxImportRows))
		}
	}
	if len(out.Rows) == 0 {
		return nil, invalid("file", "That file has headings but no shipments under them.")
	}
	return out, nil
}

func upperFirst(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

// BookImport books every shipment of a checked import, in the file's
// order. A file with a bad row books nothing. If a booking is refused all
// the same (the account turned read-only meanwhile, say), it stops there
// and returns what was booked before it with the error.
func (s *ShipmentService) BookImport(ctx context.Context, a Actor, im *Import) ([]db.Shipment, error) {
	if im.Bad > 0 {
		return nil, invalid("file", "Correct the rows marked below, then choose the file again.")
	}
	booked := make([]db.Shipment, 0, len(im.Rows))
	for _, row := range im.Rows {
		sh, err := s.Create(ctx, a, row.Input)
		if err != nil {
			if de, ok := AsError(err); ok && len(booked) > 0 {
				err = &Error{Kind: de.Kind, Message: fmt.Sprintf("Booked %d of %d shipments, then stopped at line %d: %s Remove the lines already booked before importing the rest.",
					len(booked), len(im.Rows), row.Line, de.Message)}
			}
			return booked, err
		}
		booked = append(booked, sh)
	}
	s.audit(ctx, s.q, a, "shipments_imported", "tenant", a.TenantID, nil, map[string]any{"shipments": len(booked)})
	return booked, nil
}
