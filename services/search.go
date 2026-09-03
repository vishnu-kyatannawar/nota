package services

import (
	"time"

	"github.com/vishnu-kyatannawar/nota/internal/index"
	"github.com/vishnu-kyatannawar/nota/internal/mdnote"
)

// SearchService answers the label and full-text queries the index exists for.
type SearchService struct {
	core *Core
}

// NewSearchService returns the service bound as SearchService.
func NewSearchService(core *Core) *SearchService { return &SearchService{core: core} }

// Search runs a full-text query over note contents.
func (s *SearchService) Search(query string) ([]index.Hit, error) {
	hits, err := s.core.index.Search(query)
	if err != nil {
		return nil, err
	}
	if hits == nil {
		hits = []index.Hit{}
	}
	return hits, nil
}

// Labels returns every label with how many places it is used.
func (s *SearchService) Labels() ([]index.Label, error) {
	labels, err := s.core.index.Labels()
	if err != nil {
		return nil, err
	}
	if labels == nil {
		labels = []index.Label{}
	}
	return labels, nil
}

// NotesByLabel returns the notes carrying a label.
func (s *SearchService) NotesByLabel(name string) ([]string, error) {
	paths, err := s.core.index.NotesByLabel(name)
	if err != nil {
		return nil, err
	}
	if paths == nil {
		paths = []string{}
	}
	return paths, nil
}

// HoursSummary is the time worked over a date range.
type HoursSummary struct {
	From    string `json:"from"`
	To      string `json:"to"`
	Minutes int    `json:"minutes"`
	Hours   string `json:"hours"`
}

// HoursBetween totals the hours worked over an inclusive date range. Because
// hours are one field per note, this is a single query rather than a tree scan.
func (s *SearchService) HoursBetween(from, to string) (HoursSummary, error) {
	minutes, err := s.core.index.MinutesBetween(from, to)
	if err != nil {
		return HoursSummary{}, err
	}
	return HoursSummary{From: from, To: to, Minutes: minutes, Hours: mdnote.FormatDuration(minutes)}, nil
}

// HoursThisWeek totals the current week, Monday to Sunday.
func (s *SearchService) HoursThisWeek() (HoursSummary, error) {
	now := time.Now()
	offset := (int(now.Weekday()) + 6) % 7 // Monday is the first day
	monday := now.AddDate(0, 0, -offset)
	return s.HoursBetween(monday.Format("2006-01-02"), monday.AddDate(0, 0, 6).Format("2006-01-02"))
}
