package filter

import (
	"regexp"
	"strings"

	"github.com/timastras9/joy_search_engine/pkg/models"
)

// ContentFilter filters out negative content, mugshots, and defamatory material
type ContentFilter struct {
	blockedKeywords      []string
	mugshotDetectorEnabled bool
	defamationFilterEnabled bool
	mugshotPatterns      []*regexp.Regexp
	defamationPatterns   []*regexp.Regexp
}

// NewContentFilter creates a new content filter
func NewContentFilter(blockedKeywords []string, enableMugshot, enableDefamation bool) *ContentFilter {
	cf := &ContentFilter{
		blockedKeywords:         blockedKeywords,
		mugshotDetectorEnabled:  enableMugshot,
		defamationFilterEnabled: enableDefamation,
	}

	// Compile mugshot detection patterns
	if enableMugshot {
		cf.mugshotPatterns = []*regexp.Regexp{
			regexp.MustCompile(`(?i)\bmugshot\b`),
			regexp.MustCompile(`(?i)\barrested\s+(for|on)\b`),
			regexp.MustCompile(`(?i)\bbooked\s+into\s+jail\b`),
			regexp.MustCompile(`(?i)\barrest\s+photo\b`),
			regexp.MustCompile(`(?i)\bcounty\s+jail\s+booking\b`),
		}
	}

	// Compile defamation detection patterns
	if enableDefamation {
		cf.defamationPatterns = []*regexp.Regexp{
			regexp.MustCompile(`(?i)\baccused\s+of\b`),
			regexp.MustCompile(`(?i)\balleged(ly)?\b`),
			regexp.MustCompile(`(?i)\bscandal\b`),
			regexp.MustCompile(`(?i)\bcontroversy\b`),
			regexp.MustCompile(`(?i)\blawsuit\s+(against|filed)\b`),
			regexp.MustCompile(`(?i)\bdefamation\b`),
			regexp.MustCompile(`(?i)\blibel\b`),
			regexp.MustCompile(`(?i)\bslander\b`),
			regexp.MustCompile(`(?i)\bfraud\b`),
			regexp.MustCompile(`(?i)\bcriminal\s+charge\b`),
		}
	}

	return cf
}

// FilterDocument checks if document should be filtered out
// Returns true if document should be BLOCKED, false if it should be ALLOWED
func (cf *ContentFilter) FilterDocument(doc *models.Document) (bool, string) {
	text := strings.ToLower(doc.Title + " " + doc.Content + " " + doc.Description)

	// Check for blocked keywords
	for _, keyword := range cf.blockedKeywords {
		if strings.Contains(text, strings.ToLower(keyword)) {
			return true, "blocked keyword: " + keyword
		}
	}

	// Check for mugshot content
	if cf.mugshotDetectorEnabled {
		for _, pattern := range cf.mugshotPatterns {
			if pattern.MatchString(text) {
				return true, "mugshot content detected"
			}
		}

		// Check image URLs for mugshot indicators
		for _, imgURL := range doc.Images {
			if strings.Contains(strings.ToLower(imgURL), "mugshot") ||
				strings.Contains(strings.ToLower(imgURL), "booking") {
				return true, "mugshot image detected"
			}
		}
	}

	// Check for defamatory content
	if cf.defamationFilterEnabled {
		matchCount := 0
		for _, pattern := range cf.defamationPatterns {
			if pattern.MatchString(text) {
				matchCount++
			}
		}
		// If multiple defamatory patterns match, likely defamatory content
		if matchCount >= 2 {
			return true, "potentially defamatory content"
		}
	}

	return false, ""
}

// ContainsPositiveIndicators checks for positive content indicators
func (cf *ContentFilter) ContainsPositiveIndicators(text string) bool {
	positiveKeywords := []string{
		"success", "achievement", "celebration", "happy", "joy",
		"innovation", "breakthrough", "award", "winner", "victory",
		"helping", "charity", "volunteer", "community", "kindness",
		"inspiring", "uplifting", "positive", "good news", "heartwarming",
		"recovery", "improvement", "progress", "milestone", "accomplishment",
	}

	textLower := strings.ToLower(text)
	matchCount := 0

	for _, keyword := range positiveKeywords {
		if strings.Contains(textLower, keyword) {
			matchCount++
		}
	}

	// Require at least 2 positive indicators
	return matchCount >= 2
}

// AnalyzeImageURL checks if an image URL might contain unwanted content
func (cf *ContentFilter) AnalyzeImageURL(url string) bool {
	urlLower := strings.ToLower(url)

	unwantedPatterns := []string{
		"mugshot", "booking", "arrest", "jail",
		"crime", "police", "wanted",
	}

	for _, pattern := range unwantedPatterns {
		if strings.Contains(urlLower, pattern) {
			return true // Block this image
		}
	}

	return false
}
