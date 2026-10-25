package etapi

import "context"

// Calendar endpoints create the requested note when it does not exist yet.

// Inbox calls GET /inbox/{date} (date is YYYY-MM-DD).
func (c *Client) Inbox(ctx context.Context, date string) (*Note, error) {
	return c.calendarNote(ctx, "/inbox/"+esc(date))
}

// Day calls GET /calendar/days/{date}.
func (c *Client) Day(ctx context.Context, date string) (*Note, error) {
	return c.calendarNote(ctx, "/calendar/days/"+esc(date))
}

// WeekFirstDay calls GET /calendar/week-first-day/{date}.
func (c *Client) WeekFirstDay(ctx context.Context, date string) (*Note, error) {
	return c.calendarNote(ctx, "/calendar/week-first-day/"+esc(date))
}

// Week calls GET /calendar/weeks/{week} (week is YYYY-Www).
func (c *Client) Week(ctx context.Context, week string) (*Note, error) {
	return c.calendarNote(ctx, "/calendar/weeks/"+esc(week))
}

// Month calls GET /calendar/months/{month} (month is YYYY-MM).
func (c *Client) Month(ctx context.Context, month string) (*Note, error) {
	return c.calendarNote(ctx, "/calendar/months/"+esc(month))
}

// Year calls GET /calendar/years/{year}.
func (c *Client) Year(ctx context.Context, year string) (*Note, error) {
	return c.calendarNote(ctx, "/calendar/years/"+esc(year))
}

func (c *Client) calendarNote(ctx context.Context, path string) (*Note, error) {
	var out Note
	if err := c.getJSON(ctx, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
