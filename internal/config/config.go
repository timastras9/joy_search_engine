package config

import "time"

// Config holds application configuration
type Config struct {
	// Server settings
	ServerPort string
	ServerHost string

	// Quantum GNN MCP Server
	QuantumGNNURL string
	QuantumGNNPort string

	// Sentiment thresholds
	MinSentimentScore float64 // Minimum score to include in index (0.0-1.0)

	// Crawler settings
	CrawlDepth        int
	CrawlDelay        time.Duration
	MaxConcurrent     int
	UserAgent         string
	RespectRobotsTxt  bool

	// Index settings
	IndexPath         string
	MaxIndexSize      int64 // in bytes

	// Filter settings
	EnableMugshotDetection bool
	EnableDefamationFilter bool
	BlockedKeywords        []string
}

// Default returns default configuration
func Default() *Config {
	return &Config{
		ServerPort:        "8080",
		ServerHost:        "localhost",
		QuantumGNNURL:     "http://localhost",
		QuantumGNNPort:    "8081", // Different from search server
		MinSentimentScore: 0.6,    // Only index content with 60%+ positive sentiment
		CrawlDepth:        3,
		CrawlDelay:        time.Second * 2,
		MaxConcurrent:     10,
		UserAgent:         "JoySearchBot/1.0 (+https://github.com/timastras9/joy_search_engine)",
		RespectRobotsTxt:  true,
		IndexPath:         "./data/index",
		MaxIndexSize:      5 * 1024 * 1024 * 1024 * 1024, // 5 TB
		EnableMugshotDetection: true,
		EnableDefamationFilter: true,
		BlockedKeywords: []string{
			"mugshot", "arrested", "charged with", "convicted",
			"scandal", "controversy", "accused of", "lawsuit",
			"defamation", "libel", "slander",
		},
	}
}
