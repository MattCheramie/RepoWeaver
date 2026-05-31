package server

import (
	"net/http"
	"time"

	"github.com/mattcheramie/repoweaver/internal/store"
)

// calendarView is the data backing the editorial calendar's month grid.
type calendarView struct {
	MonthLabel  string          // e.g. "May 2026"
	MonthParam  string          // "2006-01" for the current view
	PrevParam   string          // "2006-01" for the previous month
	NextParam   string          // "2006-01" for the next month
	Weekdays    []string        // column headers
	Weeks       [][]calDay      // rows of 7 days
	Unscheduled []store.Content // draft content available to schedule
}

// calDay is a single cell in the calendar grid.
type calDay struct {
	Date    string // "2006-01-02"
	Day     int    // day of month
	InMonth bool
	Today   bool
	Items   []store.Content
}

const monthFmt = "2006-01"
const dayFmt = "2006-01-02"

// handleCalendar renders the editorial calendar for the requested ?month=YYYY-MM.
// HTMX requests (month navigation) receive just the calendar fragment.
func (s *Server) handleCalendar(w http.ResponseWriter, r *http.Request) {
	d := s.base("Calendar", "calendar")
	d.Calendar = s.buildCalendar(r.URL.Query().Get("month"))
	if r.Header.Get("HX-Request") == "true" {
		s.renderNamed(w, "calendar.html", "calendar-root", d)
		return
	}
	s.render(w, "calendar.html", d)
}

// buildCalendar assembles the month grid and the list of unscheduled drafts.
// An empty or invalid month falls back to the current month.
func (s *Server) buildCalendar(month string) *calendarView {
	now := time.Now().UTC()
	anchor, err := time.Parse(monthFmt, month)
	if err != nil {
		anchor = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	} else {
		anchor = time.Date(anchor.Year(), anchor.Month(), 1, 0, 0, 0, 0, time.UTC)
	}

	all, _ := s.store.ListContent()
	byDay := map[string][]store.Content{}
	var unscheduled []store.Content
	for _, c := range all {
		if c.ScheduledFor == nil {
			unscheduled = append(unscheduled, c)
			continue
		}
		key := c.ScheduledFor.Format(dayFmt)
		byDay[key] = append(byDay[key], c)
	}

	// Grid starts on the Sunday on/before the 1st and runs full weeks.
	start := anchor.AddDate(0, 0, -int(anchor.Weekday()))
	today := now.Format(dayFmt)
	var weeks [][]calDay
	cur := start
	for {
		week := make([]calDay, 7)
		for i := 0; i < 7; i++ {
			key := cur.Format(dayFmt)
			week[i] = calDay{
				Date:    key,
				Day:     cur.Day(),
				InMonth: cur.Month() == anchor.Month(),
				Today:   key == today,
				Items:   byDay[key],
			}
			cur = cur.AddDate(0, 0, 1)
		}
		weeks = append(weeks, week)
		// Stop once we've passed the anchor month and completed a week.
		if cur.Month() != anchor.Month() && cur.After(anchor) {
			break
		}
	}

	return &calendarView{
		MonthLabel:  anchor.Format("January 2006"),
		MonthParam:  anchor.Format(monthFmt),
		PrevParam:   anchor.AddDate(0, 0, -1).Format(monthFmt),
		NextParam:   anchor.AddDate(0, 1, 0).Format(monthFmt),
		Weekdays:    []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"},
		Weeks:       weeks,
		Unscheduled: unscheduled,
	}
}
