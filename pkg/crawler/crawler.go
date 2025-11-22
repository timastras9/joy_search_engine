package crawler

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/timastras9/joy_search_engine/pkg/models"
)

// Crawler fetches and extracts content from web pages
type Crawler struct {
	httpClient    *http.Client
	userAgent     string
	delay         time.Duration
	maxDepth      int
	maxConcurrent int
	visited       sync.Map
	semaphore     chan struct{}
}

// NewCrawler creates a new web crawler
func NewCrawler(userAgent string, delay time.Duration, maxDepth, maxConcurrent int) *Crawler {
	return &Crawler{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		userAgent:     userAgent,
		delay:         delay,
		maxDepth:      maxDepth,
		maxConcurrent: maxConcurrent,
		semaphore:     make(chan struct{}, maxConcurrent),
	}
}

// Crawl fetches a URL and extracts document content
func (c *Crawler) Crawl(targetURL string) (*models.Document, error) {
	// Rate limiting
	time.Sleep(c.delay)

	// Check if already visited
	if _, visited := c.visited.LoadOrStore(targetURL, true); visited {
		return nil, fmt.Errorf("URL already visited: %s", targetURL)
	}

	// Create request
	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", c.userAgent)

	// Fetch page
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("non-200 status code: %d", resp.StatusCode)
	}

	// Parse HTML
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	// Extract content
	document := c.extractDocument(doc, targetURL)
	return document, nil
}

// extractDocument extracts relevant content from HTML document
func (c *Crawler) extractDocument(doc *goquery.Document, targetURL string) *models.Document {
	document := &models.Document{
		URL:       targetURL,
		IndexedAt: time.Now(),
	}

	// Extract title
	document.Title = doc.Find("title").First().Text()
	document.Title = strings.TrimSpace(document.Title)

	// Extract meta description
	doc.Find("meta[name='description']").Each(func(i int, s *goquery.Selection) {
		if content, exists := s.Attr("content"); exists {
			document.Description = strings.TrimSpace(content)
		}
	})

	// Extract meta keywords
	doc.Find("meta[name='keywords']").Each(func(i int, s *goquery.Selection) {
		if content, exists := s.Attr("content"); exists {
			keywords := strings.Split(content, ",")
			for _, kw := range keywords {
				document.Keywords = append(document.Keywords, strings.TrimSpace(kw))
			}
		}
	})

	// Extract author
	doc.Find("meta[name='author']").Each(func(i int, s *goquery.Selection) {
		if content, exists := s.Attr("content"); exists {
			document.Author = strings.TrimSpace(content)
		}
	})

	// Extract main content (prioritize article, main, or body content)
	var contentBuilder strings.Builder

	// Try article tags first
	doc.Find("article").Each(func(i int, s *goquery.Selection) {
		contentBuilder.WriteString(s.Text())
		contentBuilder.WriteString(" ")
	})

	// If no article, try main
	if contentBuilder.Len() == 0 {
		doc.Find("main").Each(func(i int, s *goquery.Selection) {
			contentBuilder.WriteString(s.Text())
			contentBuilder.WriteString(" ")
		})
	}

	// If still empty, try common content divs
	if contentBuilder.Len() == 0 {
		doc.Find("div.content, div.post, div.article, div.entry").Each(func(i int, s *goquery.Selection) {
			contentBuilder.WriteString(s.Text())
			contentBuilder.WriteString(" ")
		})
	}

	// Fallback to body paragraphs
	if contentBuilder.Len() == 0 {
		doc.Find("p").Each(func(i int, s *goquery.Selection) {
			contentBuilder.WriteString(s.Text())
			contentBuilder.WriteString(" ")
		})
	}

	document.Content = strings.TrimSpace(contentBuilder.String())

	// Extract images
	doc.Find("img").Each(func(i int, s *goquery.Selection) {
		if src, exists := s.Attr("src"); exists {
			// Convert relative URLs to absolute
			if imgURL, err := url.Parse(src); err == nil {
				if baseURL, err := url.Parse(targetURL); err == nil {
					absoluteURL := baseURL.ResolveReference(imgURL)
					document.Images = append(document.Images, absoluteURL.String())
				}
			}
		}
	})

	return document
}

// CrawlBatch crawls multiple URLs concurrently
func (c *Crawler) CrawlBatch(urls []string) ([]*models.Document, []error) {
	var wg sync.WaitGroup
	documents := make([]*models.Document, len(urls))
	errors := make([]error, len(urls))

	for i, url := range urls {
		wg.Add(1)
		go func(index int, targetURL string) {
			defer wg.Done()

			// Semaphore for concurrency control
			c.semaphore <- struct{}{}
			defer func() { <-c.semaphore }()

			doc, err := c.Crawl(targetURL)
			documents[index] = doc
			errors[index] = err
		}(i, url)
	}

	wg.Wait()
	return documents, errors
}

// ExtractLinks extracts all links from a document for further crawling
func (c *Crawler) ExtractLinks(doc *goquery.Document, baseURL string) []string {
	var links []string
	linkMap := make(map[string]bool)

	base, err := url.Parse(baseURL)
	if err != nil {
		return links
	}

	doc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists {
			return
		}

		// Parse and resolve URL
		linkURL, err := url.Parse(href)
		if err != nil {
			return
		}

		absoluteURL := base.ResolveReference(linkURL)

		// Only include HTTP/HTTPS links
		if absoluteURL.Scheme != "http" && absoluteURL.Scheme != "https" {
			return
		}

		urlStr := absoluteURL.String()

		// Deduplicate
		if !linkMap[urlStr] {
			linkMap[urlStr] = true
			links = append(links, urlStr)
		}
	})

	return links
}
