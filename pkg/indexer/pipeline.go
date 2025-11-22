package indexer

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/timastras9/joy_search_engine/pkg/crawler"
	"github.com/timastras9/joy_search_engine/pkg/models"
)

// Pipeline orchestrates crawling, filtering, and indexing
type Pipeline struct {
	crawler *crawler.Crawler
	indexer *Indexer
	workers int
}

// NewPipeline creates a new indexing pipeline
func NewPipeline(crawler *crawler.Crawler, indexer *Indexer, workers int) *Pipeline {
	return &Pipeline{
		crawler: crawler,
		indexer: indexer,
		workers: workers,
	}
}

// ProcessURL crawls a URL and indexes it if it passes filters
func (p *Pipeline) ProcessURL(url string) error {
	// Crawl the URL
	doc, err := p.crawler.Crawl(url)
	if err != nil {
		return fmt.Errorf("crawl failed: %w", err)
	}

	// Index the document (filtering happens inside IndexDocument)
	if err := p.indexer.IndexDocument(doc); err != nil {
		return fmt.Errorf("indexing failed: %w", err)
	}

	return nil
}

// ProcessURLs processes multiple URLs concurrently
func (p *Pipeline) ProcessURLs(urls []string) *ProcessingResult {
	result := &ProcessingResult{
		StartTime: time.Now(),
		Total:     len(urls),
	}

	var wg sync.WaitGroup
	urlChan := make(chan string, len(urls))
	resultChan := make(chan ProcessingOutcome, len(urls))

	// Start workers
	for i := 0; i < p.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for url := range urlChan {
				outcome := ProcessingOutcome{URL: url}
				err := p.ProcessURL(url)
				if err != nil {
					outcome.Error = err.Error()
					outcome.Status = "failed"
				} else {
					outcome.Status = "indexed"
				}
				resultChan <- outcome
			}
		}()
	}

	// Send URLs to workers
	go func() {
		for _, url := range urls {
			urlChan <- url
		}
		close(urlChan)
	}()

	// Wait for completion
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	for outcome := range resultChan {
		result.Outcomes = append(result.Outcomes, outcome)
		if outcome.Status == "indexed" {
			result.Indexed++
		} else {
			result.Failed++
		}
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	return result
}

// ProcessSeedList crawls and indexes from a seed list of URLs
func (p *Pipeline) ProcessSeedList(seedURLs []string, maxDepth int) *ProcessingResult {
	result := &ProcessingResult{
		StartTime: time.Now(),
	}

	visited := make(map[string]bool)
	var mu sync.Mutex

	var processLevel func(urls []string, depth int)
	processLevel = func(urls []string, depth int) {
		if depth > maxDepth {
			return
		}

		var wg sync.WaitGroup
		nextLevel := make([]string, 0)
		var nextMu sync.Mutex

		for _, url := range urls {
			// Skip if already visited
			mu.Lock()
			if visited[url] {
				mu.Unlock()
				continue
			}
			visited[url] = true
			mu.Unlock()

			wg.Add(1)
			go func(u string) {
				defer wg.Done()

				outcome := ProcessingOutcome{URL: u}
				result.Total++

				// Crawl
				doc, err := p.crawler.Crawl(u)
				if err != nil {
					outcome.Error = fmt.Sprintf("crawl failed: %v", err)
					outcome.Status = "failed"
					mu.Lock()
					result.Outcomes = append(result.Outcomes, outcome)
					result.Failed++
					mu.Unlock()
					return
				}

				// Index
				if err := p.indexer.IndexDocument(doc); err != nil {
					outcome.Error = fmt.Sprintf("index failed: %v", err)
					outcome.Status = "filtered"
					mu.Lock()
					result.Outcomes = append(result.Outcomes, outcome)
					result.Failed++
					mu.Unlock()
				} else {
					outcome.Status = "indexed"
					mu.Lock()
					result.Outcomes = append(result.Outcomes, outcome)
					result.Indexed++
					mu.Unlock()
				}

				// Extract links for next level (if depth allows)
				if depth < maxDepth {
					// Note: Need to parse HTML again to extract links
					// For simplicity, skipping link extraction in this version
					// In production, you'd cache the parsed document
				}
			}(url)
		}

		wg.Wait()

		// Process next level
		if len(nextLevel) > 0 {
			processLevel(nextLevel, depth+1)
		}
	}

	processLevel(seedURLs, 0)

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	return result
}

// ProcessingResult contains results of batch processing
type ProcessingResult struct {
	Total     int
	Indexed   int
	Failed    int
	Outcomes  []ProcessingOutcome
	StartTime time.Time
	EndTime   time.Time
	Duration  time.Duration
}

// ProcessingOutcome represents the outcome of processing a single URL
type ProcessingOutcome struct {
	URL    string
	Status string // "indexed", "filtered", "failed"
	Error  string
}

// Summary returns a formatted summary of the processing result
func (r *ProcessingResult) Summary() string {
	successRate := 0.0
	if r.Total > 0 {
		successRate = float64(r.Indexed) / float64(r.Total) * 100
	}

	return fmt.Sprintf(`
Processing Summary:
==================
Total URLs:     %d
Indexed:        %d (%.1f%%)
Failed:         %d
Duration:       %s
Rate:           %.1f URLs/sec
`,
		r.Total,
		r.Indexed,
		successRate,
		r.Failed,
		r.Duration,
		float64(r.Total)/r.Duration.Seconds(),
	)
}

// ContinuousIndexer runs continuous indexing from a URL queue
type ContinuousIndexer struct {
	pipeline  *Pipeline
	urlQueue  chan string
	stopChan  chan bool
	isRunning bool
	mu        sync.Mutex
}

// NewContinuousIndexer creates a continuous indexer
func NewContinuousIndexer(pipeline *Pipeline, queueSize int) *ContinuousIndexer {
	return &ContinuousIndexer{
		pipeline: pipeline,
		urlQueue: make(chan string, queueSize),
		stopChan: make(chan bool),
	}
}

// Start begins continuous indexing
func (ci *ContinuousIndexer) Start() {
	ci.mu.Lock()
	if ci.isRunning {
		ci.mu.Unlock()
		return
	}
	ci.isRunning = true
	ci.mu.Unlock()

	log.Println("Starting continuous indexer...")

	go func() {
		for {
			select {
			case url := <-ci.urlQueue:
				if err := ci.pipeline.ProcessURL(url); err != nil {
					log.Printf("Error processing %s: %v", url, err)
				}
			case <-ci.stopChan:
				log.Println("Stopping continuous indexer...")
				ci.mu.Lock()
				ci.isRunning = false
				ci.mu.Unlock()
				return
			}
		}
	}()
}

// AddURL adds a URL to the indexing queue
func (ci *ContinuousIndexer) AddURL(url string) {
	ci.urlQueue <- url
}

// Stop stops the continuous indexer
func (ci *ContinuousIndexer) Stop() {
	ci.stopChan <- true
}

// IsRunning returns whether the indexer is running
func (ci *ContinuousIndexer) IsRunning() bool {
	ci.mu.Lock()
	defer ci.mu.Unlock()
	return ci.isRunning
}
