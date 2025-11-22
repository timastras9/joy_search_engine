package models

import "time"

// Document represents a web page in the index
type Document struct {
	ID             string    `json:"id"`
	URL            string    `json:"url"`
	Title          string    `json:"title"`
	Content        string    `json:"content"`
	Description    string    `json:"description"`
	SentimentScore float64   `json:"sentiment_score"` // 0.0 (negative) to 1.0 (positive)
	IndexedAt      time.Time `json:"indexed_at"`
	Author         string    `json:"author,omitempty"`
	PublishedAt    time.Time `json:"published_at,omitempty"`
	Images         []string  `json:"images,omitempty"`
	Keywords       []string  `json:"keywords,omitempty"`
}

// SearchResult represents a search query result
type SearchResult struct {
	Documents []Document `json:"documents"`
	Total     int        `json:"total"`
	Page      int        `json:"page"`
	PageSize  int        `json:"page_size"`
}

// CrawlJob represents a crawling task
type CrawlJob struct {
	URL       string    `json:"url"`
	Depth     int       `json:"depth"`
	StartedAt time.Time `json:"started_at"`
	Status    string    `json:"status"` // pending, crawling, completed, failed
}
