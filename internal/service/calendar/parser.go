package calendar

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/arkcode369/ff-calendar-bot/internal/domain"
	"github.com/arkcode369/ff-calendar-bot/pkg/timeutil"
)

// Parser handles HTML parsing of ForexFactory calendar pages.
// It extracts events from the weekly calendar table and historical
// data from individual event detail pages.
type Parser struct {
	loc *time.Location
}

// NewParser creates a parser that interprets FF times as ET,
// then converts to WIB for internal storage.
func NewParser() *Parser {
	return &Parser{
		loc: timeutil.WIB, // FIX: WIB is a var, not a func
	}
}

var (
	// Regex patterns for FF calendar HTML parsing
	reImpactClass  = regexp.MustCompile(`icon--ff-impact-(red|ora|yel|gra)`)
	reEventID      = regexp.MustCompile(`data-eventid="(\d+)"`)
	reDateTime     = regexp.MustCompile(`data-time="(\d+)"`)
	reNumericValue = regexp.MustCompile(`^[+-]?[\d,]+\.?\d*[%KMBTkmbt]?$`)
	reSpeakerTag   = regexp.MustCompile(`(?i)(speaks?|speech|testimony|conference|remarks)`)
	reAllDay       = regexp.MustCompile(`(?i)(all\s*day|tentative)`)
	reHistoryRow   = regexp.MustCompile(`<tr[^>]*class="[^"]*calendar_row[^"]*"[^>]*>`)
	reHTMLTag      = regexp.MustCompile(`<[^>]+>`)
)

// impactMap converts FF CSS class suffixes to domain impact levels.
// FIX: Use domain.ImpactLevel (not domain.EventImpactLevel)
// FIX: "gra" maps to ImpactNone (no ImpactHoliday in domain)
var impactMap = map[string]domain.ImpactLevel{
	"red": domain.ImpactHigh,
	"ora": domain.ImpactMedium,
	"yel": domain.ImpactLow,
	"gra": domain.ImpactNone, // FIX: was domain.ImpactHoliday which doesn't exist
}

// ParseWeeklyCalendarHTML extracts events from the FF weekly calendar HTML.
// The HTML is a <table> with class "calendar__table" containing rows
// where each row has: date, time, currency, impact, event name, actual, forecast, previous.
func (p *Parser) ParseWeeklyCalendarHTML(html string) ([]domain.FFEvent, error) {
	if html == "" {
		return nil, fmt.Errorf("empty HTML")
	}

	rows := splitCalendarRows(html)
	if len(rows) == 0 {
		return nil, fmt.Errorf("no calendar rows found")
	}

	var events []domain.FFEvent
	var currentDate time.Time

	for _, row := range rows {
		// Extract date if present (date rows span the full week)
		if d := extractDate(row); !d.IsZero() {
			currentDate = d
			continue
		}

		ev, err := p.parseEventRow(row, currentDate)
		if err != nil {
			continue // skip unparseable rows
		}

		events = append(events, ev)
	}

	return events, nil
}

// parseEventRow extracts a single event from an HTML table row.
func (p *Parser) parseEventRow(row string, date time.Time) (domain.FFEvent, error) {
	ev := domain.FFEvent{}

	// Event ID
	if m := reEventID.FindStringSubmatch(row); len(m) > 1 {
		ev.ID = m[1]
	}

	// Time
	ev.IsAllDay = reAllDay.MatchString(row)
	if !ev.IsAllDay {
		if m := reDateTime.FindStringSubmatch(row); len(m) > 1 {
			ts, _ := strconv.ParseInt(m[1], 10, 64)
			if ts > 0 {
				// FF uses Unix timestamp in ET
				ev.Date = time.Unix(ts, 0).In(p.loc) // FIX: was ev.DateTime
			}
		}
	}
	if ev.Date.IsZero() && !date.IsZero() { // FIX: was ev.DateTime
		ev.Date = date // FIX: was ev.DateTime
	}

	// Currency
	ev.Currency = extractCellText(row, "calendar__currency")
	if ev.Currency == "" {
		return ev, fmt.Errorf("no currency")
	}

	// Impact
	if m := reImpactClass.FindStringSubmatch(row); len(m) > 1 {
		if imp, ok := impactMap[m[1]]; ok {
			ev.Impact = imp
		}
	}

	// Event name
	ev.Title = extractCellText(row, "calendar__event")
	if ev.Title == "" {
		return ev, fmt.Errorf("no title")
	}

	// Categorization
	ev.Category = categorizeEvent(ev.Title)
	if reSpeakerTag.MatchString(ev.Title) {
		ev.Category = domain.CategorySpeech
		ev.SpeakerName = extractSpeaker(ev.Title)
	}

	// Data values
	ev.Actual = extractCellText(row, "calendar__actual")
	ev.Forecast = extractCellText(row, "calendar__forecast")
	ev.Previous = extractCellText(row, "calendar__previous")

	// Detect revision in previous (FF strikes through original value)
	if strings.Contains(row, "revised") || strings.Contains(row, "<s>") {
		ev.Revision = extractRevision(row, ev.Previous)
	}

	// Source metadata
	ev.ScrapedAt = time.Now()
	ev.Source = "forexfactory"

	return ev, nil
}

// ---------------------------------------------------------------------------
// History Parser
// ---------------------------------------------------------------------------

// ParseEventHistoryHTML extracts historical data points from an FF event detail page.
func (p *Parser) ParseEventHistoryHTML(html string, eventName, currency string) ([]domain.FFEventDetail, error) {
	if html == "" {
		return nil, fmt.Errorf("empty HTML")
	}

	rows := reHistoryRow.FindAllString(html, -1)
	if len(rows) == 0 {
		return nil, fmt.Errorf("no history rows found")
	}

	var details []domain.FFEventDetail

	for _, row := range rows {
		d := domain.FFEventDetail{
			EventName: eventName,
			Currency:  currency,
		}

		// Date
		dateStr := extractCellText(row, "calendar__date")
		if t, err := timeutil.ParseFFDate(dateStr); err == nil {
			d.Date = t
		} else {
			continue // skip rows without valid dates
		}

		// Numeric values
		d.Actual = parseNumeric(extractCellText(row, "calendar__actual"))
		d.Forecast = parseNumeric(extractCellText(row, "calendar__forecast"))
		d.Previous = parseNumeric(extractCellText(row, "calendar__previous"))
		d.Surprise = d.Actual - d.Forecast

		// Revision detection
		if strings.Contains(row, "revised") {
			d.Revised = parseNumeric(extractRevisedValue(row))
		}

		details = append(details, d)
	}

	return details, nil
}

// ---------------------------------------------------------------------------
// Helper functions
// ---------------------------------------------------------------------------

// splitCalendarRows splits the calendar HTML into individual row strings.
func splitCalendarRows(html string) []string {
	re := regexp.MustCompile(`<tr[^>]*class="[^"]*calendar__row[^"]*"[^>]*>(.*?)</tr>`)
	matches := re.FindAllString(html, -1)
	return matches
}

// extractDate extracts a date from a date row header.
func extractDate(row string) time.Time {
	re := regexp.MustCompile(`data[-_]date="([^"]+)"`)
	m := re.FindStringSubmatch(row)
	if len(m) > 1 {
		t, err := timeutil.ParseFFDate(m[1])
		if err == nil {
			return t
		}
	}
	return time.Time{}
}

// extractCellText extracts the text content from a <td> with the given CSS class.
func extractCellText(row, className string) string {
	pattern := fmt.Sprintf(`<td[^>]*class="[^"]*%s[^1]*"[^>]*>(.*?)</td>`, regexp.QuoteMeta(className))
	re := regexp.MustCompile(pattern)
	m := re.FindStringSubmatch(row)
	if len(m) > 1 {
		// Strip HTML tags
		text := reHTMLTag.ReplaceAllString(m[1], "")
		return strings.TrimSpace(text)
	}
	return ""
}

// extractSpeaker extracts the speaker name from an event title.
// Example: "FED Chair Powell Speaks" -> "Powell"
func extractSpeaker(title string) string {
	re := regexp.MustCompile(`(<i)(\w+)\s+(speaks?|testimony|conference|remarks)`)
	m := re.FindStringSubmatch(title)
	if len(m) > 2 {
		return m[2]
	}
	return ""
}

// extractRevision detects and extracts a previous value revision.
func extractRevision(row string, currentPrev string) *domain.EventRevision {
	// Look for strikethrough tag containing original value
	re := regexp.MustCompile(`<s>([^<]+)</s>`)
	m := re.FindStringSubmatch(row)
	if len(m) > 1 {
		original := strings.TrimSpace(m[1])
		rev := &domain.EventRevision{
			OriginalValue: original,
			RevisedValue:  currentPrev,
			Direction:     detectRevisionDirection(original, currentPrev),
		}
		rev.Magnitude = math.Abs(parseNumeric(currentPrev) - parseNumeric(original)) // FIX: added "math" import
		return rev
	}
	return nil
}

// extractRevisedValue extracts the revised value from a history row.
func extractRevisedValue(row string) string {
	// Look for value after strikethrough
	re := regexp.MustCompile(`<s>[^<]+</s>\s*([^ <]+)`)
	m := re.FindStringSubmatch(row)
	if len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

// detectRevisionDirection compares original and revised values.
func detectRevisionDirection(original, revised string) domain.RevisionDirection {
	origNum := parseNumeric(original)
	revNum := parseNumeric(revised)
	if revNum > origNum {
		return domain.RevisionUp
	}
	if revNum < origNum {
		return domain.RevisionDown
	}
	return domain.RevisionFlat
}

// parseNumeric converts a formatted numeric string to float64.
// Handles: "450K" (450000), "2.5%" (2.5), "-3.2M" (-3200000), "1,235" (1235)
func parseNumeric(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" || s == "N/A" {
		return 0
	}

	// Remove percent and commas
	s = strings.ReplaceAll(s, "%", "")
	s = strings.ReplaceAll(s, ",", "")

	// Check for suffix multipliers
	multiplier := 1.0
	if strings.HasSuffix(s, "K") || strings.HasSuffix(s, "k") {
		multiplier = 1000
		s = s[:len(s)-1]
	} else if strings.HasSuffix(s, "M") || strings.HasSuffix(s, "m") {
		multiplier = 1000000
		s = s[:len(s)-1]
	} else if strings.HasSuffix(s, "B") || strings.HasSuffix(s, "b") {
		multiplier = 1000000000
		s = s[:len(s)-1]
	} else if strings.HasSuffix(s, "T") || strings.HasSuffix(s, "t") {
		multiplier = 1000000000000
		s = s[:len(s)-1]
	}

	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return n * multiplier
}

// categorizeEvent returns the event category based on the title keywords.
func categorizeEvent(title string) domain.EventCategory {
	lower := strings.ToLower(title)

	switch {
	case containsAny(lower, "rate decision", "ministers meeting", "ministres", "minintes", "mpc", "fomc", "statement rate"):
		return domain.CategoryCentralBank
	case containsAny(lower, "speak", "speech", "testimony", "conference", "remarks"):
		return domain.CategorySpeech
	case containsAny(lower, "auction", "bond"):
		return domain.CategoryAuction
	case containsAny(lower, "holiday", "bank holiday"):
		return domain.CategoryHoliday
	default:
		return domain.CategoryEconomicIndicator
	}
}

// containsAny checks if s contains any of the substrings.
func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
