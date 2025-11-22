package indexer

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/mapping"
	"github.com/timastras9/joy_search_engine/internal/config"
	"github.com/timastras9/joy_search_engine/pkg/filter"
	"github.com/timastras9/joy_search_engine/pkg/models"
	"github.com/timastras9/joy_search_engine/pkg/sentiment"
)

// Indexer manages the search index with positive content filtering
type Indexer struct {
	index            bleve.Index
	sentimentAnalyzer *sentiment.Analyzer
	contentFilter    *filter.ContentFilter
	config           *config.Config
	indexPath        string
	mu               sync.RWMutex
	stats            IndexStats
}

// IndexStats tracks indexing statistics
type IndexStats struct {
	TotalProcessed    int64
	TotalIndexed      int64
	TotalFiltered     int64
	FilteredByContent int64
	FilteredBySentiment int64
	LastUpdated      time.Time
}

// NewIndexer creates a new search indexer
func NewIndexer(cfg *config.Config, sentimentAnalyzer *sentiment.Analyzer, contentFilter *filter.ContentFilter) (*Indexer, error) {
	indexPath := cfg.IndexPath

	var index bleve.Index
	var err error

	// Check if index already exists
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		// Create new index
		indexMapping := buildIndexMapping()
		index, err = bleve.New(indexPath, indexMapping)
		if err != nil {
			return nil, fmt.Errorf("failed to create index: %w", err)
		}
		log.Printf("Created new index at %s", indexPath)
	} else {
		// Open existing index
		index, err = bleve.Open(indexPath)
		if err != nil {
			return nil, fmt.Errorf("failed to open index: %w", err)
		}
		log.Printf("Opened existing index at %s", indexPath)
	}

	return &Indexer{
		index:            index,
		sentimentAnalyzer: sentimentAnalyzer,
		contentFilter:    contentFilter,
		config:           cfg,
		indexPath:        indexPath,
		stats:            IndexStats{LastUpdated: time.Now()},
	}, nil
}

// buildIndexMapping creates the Bleve index mapping
func buildIndexMapping() mapping.IndexMapping {
	// Create a custom mapping
	indexMapping := bleve.NewIndexMapping()

	// Document mapping
	docMapping := bleve.NewDocumentMapping()

	// Text fields with full-text analysis
	textFieldMapping := bleve.NewTextFieldMapping()
	textFieldMapping.Analyzer = "en"
	textFieldMapping.Store = true
	textFieldMapping.Index = true

	// Keyword fields (not analyzed)
	keywordFieldMapping := bleve.NewKeywordFieldMapping()
	keywordFieldMapping.Store = true
	keywordFieldMapping.Index = true

	// Numeric fields
	numericFieldMapping := bleve.NewNumericFieldMapping()
	numericFieldMapping.Store = true
	numericFieldMapping.Index = true

	// Date/time fields
	dateFieldMapping := bleve.NewDateTimeFieldMapping()
	dateFieldMapping.Store = true
	dateFieldMapping.Index = true

	// Map document fields
	docMapping.AddFieldMappingsAt("title", textFieldMapping)
	docMapping.AddFieldMappingsAt("content", textFieldMapping)
	docMapping.AddFieldMappingsAt("description", textFieldMapping)
	docMapping.AddFieldMappingsAt("url", keywordFieldMapping)
	docMapping.AddFieldMappingsAt("author", textFieldMapping)
	docMapping.AddFieldMappingsAt("sentiment_score", numericFieldMapping)
	docMapping.AddFieldMappingsAt("indexed_at", dateFieldMapping)
	docMapping.AddFieldMappingsAt("published_at", dateFieldMapping)
	docMapping.AddFieldMappingsAt("keywords", textFieldMapping)

	indexMapping.AddDocumentMapping("document", docMapping)
	indexMapping.DefaultMapping = docMapping

	return indexMapping
}

// IndexDocument indexes a document after applying filters
func (idx *Indexer) IndexDocument(doc *models.Document) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	idx.stats.TotalProcessed++

	// Step 1: Apply content filter
	blocked, reason := idx.contentFilter.FilterDocument(doc)
	if blocked {
		idx.stats.TotalFiltered++
		idx.stats.FilteredByContent++
		return fmt.Errorf("document blocked by content filter: %s", reason)
	}

	// Step 2: Analyze sentiment using quantum GNN
	sentimentResult, err := idx.sentimentAnalyzer.AnalyzeText(doc.Title + " " + doc.Content)
	if err != nil {
		return fmt.Errorf("sentiment analysis failed: %w", err)
	}

	doc.SentimentScore = sentimentResult.Score

	// Step 3: Filter by sentiment threshold
	if doc.SentimentScore < idx.config.MinSentimentScore {
		idx.stats.TotalFiltered++
		idx.stats.FilteredBySentiment++
		return fmt.Errorf("document sentiment score too low: %.2f < %.2f",
			doc.SentimentScore, idx.config.MinSentimentScore)
	}

	// Step 4: Generate document ID
	doc.ID = generateDocumentID(doc.URL)
	doc.IndexedAt = time.Now()

	// Step 5: Index the document
	if err := idx.index.Index(doc.ID, doc); err != nil {
		return fmt.Errorf("failed to index document: %w", err)
	}

	idx.stats.TotalIndexed++
	idx.stats.LastUpdated = time.Now()

	log.Printf("Indexed: %s (sentiment: %.2f, title: %s)", doc.URL, doc.SentimentScore, doc.Title)
	return nil
}

// IndexBatch indexes multiple documents concurrently with filtering
func (idx *Indexer) IndexBatch(docs []*models.Document) (int, []error) {
	var wg sync.WaitGroup
	results := make(chan error, len(docs))

	for _, doc := range docs {
		wg.Add(1)
		go func(d *models.Document) {
			defer wg.Done()
			err := idx.IndexDocument(d)
			results <- err
		}(doc)
	}

	wg.Wait()
	close(results)

	// Collect results
	var errors []error
	successCount := 0
	for err := range results {
		if err != nil {
			errors = append(errors, err)
		} else {
			successCount++
		}
	}

	return successCount, errors
}

// Search performs a search query on the index
func (idx *Indexer) Search(queryString string, page, pageSize int) (*models.SearchResult, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	// Build query
	query := bleve.NewQueryStringQuery(queryString)
	searchRequest := bleve.NewSearchRequest(query)

	// Pagination
	searchRequest.From = page * pageSize
	searchRequest.Size = pageSize

	// Sort by sentiment score (descending) and relevance
	searchRequest.SortBy([]string{"-sentiment_score", "-_score"})

	// Highlight matches
	searchRequest.Highlight = bleve.NewHighlight()

	// Execute search
	searchResult, err := idx.index.Search(searchRequest)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	// Build result
	documents := make([]models.Document, 0, len(searchResult.Hits))
	for _, hit := range searchResult.Hits {
		var doc models.Document
		if err := idx.index.Document(hit.ID, &doc); err != nil {
			log.Printf("Failed to retrieve document %s: %v", hit.ID, err)
			continue
		}
		documents = append(documents, doc)
	}

	result := &models.SearchResult{
		Documents: documents,
		Total:     int(searchResult.Total),
		Page:      page,
		PageSize:  pageSize,
	}

	return result, nil
}

// SearchByURL searches for a document by exact URL
func (idx *Indexer) SearchByURL(url string) (*models.Document, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	docID := generateDocumentID(url)
	var doc models.Document

	err := idx.index.Document(docID, &doc)
	if err != nil {
		return nil, fmt.Errorf("document not found: %w", err)
	}

	return &doc, nil
}

// DeleteDocument removes a document from the index
func (idx *Indexer) DeleteDocument(url string) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	docID := generateDocumentID(url)
	if err := idx.index.Delete(docID); err != nil {
		return fmt.Errorf("failed to delete document: %w", err)
	}

	log.Printf("Deleted document: %s", url)
	return nil
}

// GetStats returns current indexing statistics
func (idx *Indexer) GetStats() IndexStats {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.stats
}

// GetIndexCount returns the total number of documents in the index
func (idx *Indexer) GetIndexCount() (uint64, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	count, err := idx.index.DocCount()
	if err != nil {
		return 0, fmt.Errorf("failed to get document count: %w", err)
	}

	return count, nil
}

// Close closes the index
func (idx *Indexer) Close() error {
	return idx.index.Close()
}

// Optimize optimizes the index for better search performance
func (idx *Indexer) Optimize() error {
	log.Println("Optimizing index...")
	// Bleve doesn't have explicit optimization, but we can close and reopen
	// In production, you might want to implement index compaction
	return nil
}

// generateDocumentID creates a unique ID from URL
func generateDocumentID(url string) string {
	hash := sha256.Sum256([]byte(url))
	return hex.EncodeToString(hash[:])
}
