package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	badger "github.com/dgraph-io/badger/v4"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// NewsRepo implements ports.NewsRepository using BadgerDB.
type NewsRepo struct {
	db *badger.DB
}

// NewNewsRepo creates a new NewsRepo backed by the given DB.
func NewNewsRepo(db *DB) *NewsRepo {
	return &NewsRepo{db: db.Badger()}
}

// --- Key builders ---

// newsKey formatting: news:{date}:{id} -> "news:20260317:some-hash"
func newsKey(date string, eventID string) []byte {
	return []byte(fmt.Sprintf("news:%s:%s", date, eventID))
}

// newsPrefix formatting: news:{date}:
func newsPrefix(date string) []byte {
	return []byte(fmt.Sprintf("news:%s:", date))
}

// SaveEvents stores a batch of NewsEvent records.
func (r *NewsRepo) SaveEvents(_ context.Context, events []domain.NewsEvent) error {
	wb := r.db.NewWriteBatch()
	defer wb.Cancel()

	for i := range events {
		data, err := json.Marshal(&events[i])
		if err != nil {
			return fmt.Errorf("marshal news event %s: %w", events[i].ID, err)
		}
		// Assuming domain.NewsEvent.Date is formatted properly
		// For simplicity, we can use TimeWIB to format a solid "20060102"
		dateKeyStr := events[i].TimeWIB.Format("20060102")
		key := newsKey(dateKeyStr, events[i].ID)

		if err := wb.Set(key, data); err != nil {
			return fmt.Errorf("batch set news %s: %w", events[i].ID, err)
		}
	}

	if err := wb.Flush(); err != nil {
		return fmt.Errorf("flush news batch: %w", err)
	}
	return nil
}

// GetByDate returns all events for a specific date "YYYYMMDD".
func (r *NewsRepo) GetByDate(_ context.Context, date string) ([]domain.NewsEvent, error) {
	var events []domain.NewsEvent
	prefix := newsPrefix(date)

	err := r.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix
		opts.PrefetchValues = true
		opts.PrefetchSize = 50

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				var evt domain.NewsEvent
				if err := json.Unmarshal(val, &evt); err != nil {
					return err
				}
				events = append(events, evt)
				return nil
			})
			if err != nil {
				return fmt.Errorf("read news at %s: %w", item.Key(), err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("get news by date: %w", err)
	}
	return events, nil
}

// GetByWeek returns events starting from a specific date spanning 7 days.
func (r *NewsRepo) GetByWeek(ctx context.Context, weekStart string) ([]domain.NewsEvent, error) {
	var allEvents []domain.NewsEvent

	// BUG #3 FIX: Parse with WIB timezone to avoid off-by-one at midnight boundary.
	// time.Parse() always returns UTC — on a WIB machine the day boundary can shift
	// by ±7 hours, causing the week to start on the wrong date.
	wibLoc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		// Fallback: construct WIB as UTC+7 if the timezone database is unavailable.
		wibLoc = time.FixedZone("WIB", 7*60*60)
	}
	startT, err := time.ParseInLocation("20060102", weekStart, wibLoc)
	if err != nil {
		return nil, fmt.Errorf("invalid weekStart format: %w", err)
	}

	for startT.Weekday() != time.Monday {
		startT = startT.AddDate(0, 0, -1)
	}

	// Iterate for 7 days
	for i := 0; i < 7; i++ {
		dateStr := startT.AddDate(0, 0, i).Format("20060102")
		dailyEvents, err := r.GetByDate(ctx, dateStr)
		if err != nil {
			return nil, err
		}
		allEvents = append(allEvents, dailyEvents...)
	}

	return allEvents, nil
}

// GetByMonth returns all events for a given month. yearMonth format: "202603"
func (r *NewsRepo) GetByMonth(ctx context.Context, yearMonth string) ([]domain.NewsEvent, error) {
	// yearMonth is "YYYYMM" — scan all dates with prefix "news:YYYYMM"
	var allEvents []domain.NewsEvent
	prefix := []byte(fmt.Sprintf("news:%s", yearMonth))

	err := r.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix
		opts.PrefetchValues = true
		opts.PrefetchSize = 100

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				var evt domain.NewsEvent
				if err := json.Unmarshal(val, &evt); err != nil {
					return err
				}
				allEvents = append(allEvents, evt)
				return nil
			})
			if err != nil {
				return fmt.Errorf("read news at %s: %w", item.Key(), err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("get news by month: %w", err)
	}
	return allEvents, nil
}

// GetPending returns all events for a specific date where status is "pending_retry".
func (r *NewsRepo) GetPending(ctx context.Context, date string) ([]domain.NewsEvent, error) {
	daily, err := r.GetByDate(ctx, date)
	if err != nil {
		return nil, err
	}

	var pending []domain.NewsEvent
	for _, e := range daily {
		// BUG #4 FIX: Added explicit parentheses to clarify operator precedence.
		// Without them, Go evaluates as: e.Status=="pending_retry" || (e.Status=="upcoming" && e.Actual=="")
		// which is accidentally correct in Go (&&  binds tighter than ||), but the intent
		// should be explicit to avoid misreads and future regressions.
		if e.Status == "pending_retry" || (e.Status == "upcoming" && e.Actual == "") {
			// Include events that still need fetching
			pending = append(pending, e)
		}
	}
	return pending, nil
}

// UpdateActual updates a specific event's Actual field.
// It first attempts an O(1) direct lookup using today's date extracted from the event ID
// prefix "mql5-{numericID}". If the event is not found on today's date, it falls back to
// scanning only the last 3 days (not the entire table) to handle events near midnight
// boundary or delayed releases.
func (r *NewsRepo) UpdateActual(ctx context.Context, id string, actual string) error {
	// Strategy: try direct key lookup for today and the previous 2 days before
	// falling back to a bounded 3-day scan. This reduces the worst case from
	// O(total events) to O(events in 3 days) — typically < 200 records.
	now := time.Now().UTC()
	for offset := 0; offset <= 2; offset++ {
		dateStr := now.AddDate(0, 0, -offset).Format("20060102")
		key := newsKey(dateStr, id)

		var evt domain.NewsEvent
		found := false

		err := r.db.View(func(txn *badger.Txn) error {
			item, err := txn.Get(key)
			if err == badger.ErrKeyNotFound {
				return nil // not on this day, try next
			}
			if err != nil {
				return err
			}
			found = true
			return item.Value(func(val []byte) error {
				return json.Unmarshal(val, &evt)
			})
		})
		if err != nil {
			return fmt.Errorf("lookup event %s on %s: %w", id, dateStr, err)
		}
		if !found {
			continue
		}

		// Found — update and write back
		evt.Actual = actual
		if actual != "" {
			evt.Status = "released"
		}
		data, err := json.Marshal(&evt)
		if err != nil {
			return fmt.Errorf("marshal event for update actual: %w", err)
		}
		return r.db.Update(func(txn *badger.Txn) error {
			return txn.Set(key, data)
		})
	}

	// Fallback: scan only the last 3 days (bounded O(n) for ~200 records max)
	// This handles edge cases like events stored under an unexpected date key.
	return r.updateActualFallback(ctx, id, actual)
}

// updateActualFallback scans news events for the last 3 days to find and update an event.
// Called only when the direct-key lookup in UpdateActual fails.
func (r *NewsRepo) updateActualFallback(_ context.Context, id string, actual string) error {
	now := time.Now().UTC()
	for offset := 0; offset <= 2; offset++ {
		dateStr := now.AddDate(0, 0, -offset).Format("20060102")
		prefix := newsPrefix(dateStr)

		var targetKey []byte
		var evt domain.NewsEvent

		err := r.db.View(func(txn *badger.Txn) error {
			opts := badger.DefaultIteratorOptions
			opts.Prefix = prefix
			opts.PrefetchValues = true
			opts.PrefetchSize = 50

			it := txn.NewIterator(opts)
			defer it.Close()

			for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
				item := it.Item()
				if strings.HasSuffix(string(item.Key()), ":"+id) {
					targetKey = item.KeyCopy(nil)
					return item.Value(func(val []byte) error {
						return json.Unmarshal(val, &evt)
					})
				}
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("fallback scan on %s: %w", dateStr, err)
		}
		if targetKey == nil {
			continue
		}

		evt.Actual = actual
		if actual != "" {
			evt.Status = "released"
		}
		data, err := json.Marshal(&evt)
		if err != nil {
			return fmt.Errorf("marshal event fallback: %w", err)
		}
		return r.db.Update(func(txn *badger.Txn) error {
			return txn.Set(targetKey, data)
		})
	}

	return fmt.Errorf("event %s not found in last 3 days", id)
}

// UpdateStatus updates the event status using the same bounded-lookup strategy as UpdateActual.
func (r *NewsRepo) UpdateStatus(_ context.Context, id string, status string, retryCount int) error {
	now := time.Now().UTC()
	for offset := 0; offset <= 2; offset++ {
		dateStr := now.AddDate(0, 0, -offset).Format("20060102")
		key := newsKey(dateStr, id)

		var evt domain.NewsEvent
		found := false

		err := r.db.View(func(txn *badger.Txn) error {
			item, err := txn.Get(key)
			if err == badger.ErrKeyNotFound {
				return nil
			}
			if err != nil {
				return err
			}
			found = true
			return item.Value(func(val []byte) error {
				return json.Unmarshal(val, &evt)
			})
		})
		if err != nil {
			return fmt.Errorf("lookup event %s on %s: %w", id, dateStr, err)
		}
		if !found {
			continue
		}

		evt.Status = status
		evt.RetryCount = retryCount
		data, err := json.Marshal(&evt)
		if err != nil {
			return fmt.Errorf("marshal event for update status: %w", err)
		}
		return r.db.Update(func(txn *badger.Txn) error {
			return txn.Set(key, data)
		})
	}
	return fmt.Errorf("event %s not found in last 3 days for status update", id)
}

// GetHistoricalSurprises returns a slice of (actual - forecast) raw differences for
// a given event name + currency pair, looking back up to lookbackMonths months.
// It scans monthly prefixes in BadgerDB and collects past releases where both
// Actual and Forecast are non-empty. Returns nil (not an error) if < 3 data points exist.
func (r *NewsRepo) GetHistoricalSurprises(_ context.Context, eventName string, currency string, lookbackMonths int) ([]float64, error) {
	var diffs []float64

	now := time.Now()
	for i := 1; i <= lookbackMonths; i++ {
		// Walk backwards month by month
		t := now.AddDate(0, -i, 0)
		yearMonth := t.Format("200601") // e.g. "202602"
		prefix := []byte(fmt.Sprintf("news:%s", yearMonth))

		scanErr := r.db.View(func(txn *badger.Txn) error {
			opts := badger.DefaultIteratorOptions
			opts.Prefix = prefix
			opts.PrefetchValues = true
			opts.PrefetchSize = 100

			it := txn.NewIterator(opts)
			defer it.Close()

			for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
				item := it.Item()
				valErr := item.Value(func(val []byte) error {
					var evt domain.NewsEvent
					if err := json.Unmarshal(val, &evt); err != nil {
						return nil // skip malformed records
					}
					// Match by event name and currency
					if evt.Event != eventName || evt.Currency != currency {
						return nil
					}
					if evt.Actual == "" || evt.Forecast == "" {
						return nil
					}
					// Parse numeric values inline — avoid import cycle by parsing here
					actualVal, aOk := parseSimpleFloat(evt.Actual)
					forecastVal, fOk := parseSimpleFloat(evt.Forecast)
					if !aOk || !fOk {
						return nil
					}
					diffs = append(diffs, actualVal-forecastVal)
					return nil
				})
				if valErr != nil {
					return valErr
				}
			}
			return nil
		})
		if scanErr != nil {
			// Non-fatal: partial results are acceptable
			continue
		}
	}

	if len(diffs) < 3 {
		return nil, nil // not enough history — caller should use raw diff
	}
	return diffs, nil
}

// parseSimpleFloat parses a numeric string that may contain %, K, M, B suffixes or commas.
// This is a local helper used by GetHistoricalSurprises to avoid import cycles with
// the service layer (which owns ParseNumericValue).
func parseSimpleFloat(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" || s == "N/A" || s == "-" {
		return 0, false
	}
	multiplier := 1.0
	switch {
	case strings.HasSuffix(s, "K") || strings.HasSuffix(s, "k"):
		multiplier = 1_000
		s = s[:len(s)-1]
	case strings.HasSuffix(s, "M") || strings.HasSuffix(s, "m"):
		multiplier = 1_000_000
		s = s[:len(s)-1]
	case strings.HasSuffix(s, "B") || strings.HasSuffix(s, "b"):
		multiplier = 1_000_000_000
		s = s[:len(s)-1]
	}
	s = strings.TrimRight(s, "%")
	s = strings.ReplaceAll(s, ",", "")
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return v * multiplier, true
}

// SaveRevision stores an event revision record for historical tracking.
func (r *NewsRepo) SaveRevision(_ context.Context, rev domain.EventRevision) error {
	data, err := json.Marshal(&rev)
	if err != nil {
		return fmt.Errorf("marshal revision: %w", err)
	}

	key := []byte(fmt.Sprintf("evtrev:%s:%s:%s",
		rev.Currency,
		rev.RevisionDate.Format("20060102"),
		rev.EventID,
	))
	return r.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, data)
	})
}
