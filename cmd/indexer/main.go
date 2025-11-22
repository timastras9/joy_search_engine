package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/timastras9/joy_search_engine/internal/config"
	"github.com/timastras9/joy_search_engine/pkg/crawler"
	"github.com/timastras9/joy_search_engine/pkg/filter"
	"github.com/timastras9/joy_search_engine/pkg/indexer"
	"github.com/timastras9/joy_search_engine/pkg/sentiment"
)

func main() {
	// Parse flags
	urlFile := flag.String("urls", "", "File containing URLs to index (one per line)")
	url := flag.String("url", "", "Single URL to index")
	workers := flag.Int("workers", 5, "Number of concurrent workers")
	maxDepth := flag.Int("depth", 0, "Maximum crawl depth (0 = no following links)")
	indexPath := flag.String("index", "./data/index", "Path to search index")
	quantumURL := flag.String("quantum-url", "http://localhost:8081", "Quantum GNN MCP server URL")
	minSentiment := flag.Float64("min-sentiment", 0.6, "Minimum sentiment score (0.0-1.0)")
	flag.Parse()

	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║           Joy Search Engine - Indexer                       ║")
	fmt.Println("║     Positive Content Only - Powered by Quantum GNN          ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Load config
	cfg := config.Default()
	cfg.IndexPath = *indexPath
	cfg.QuantumGNNURL = *quantumURL
	cfg.MinSentimentScore = *minSentiment
	cfg.MaxConcurrent = *workers

	// Initialize components
	fmt.Println("🔧 Initializing components...")

	// Sentiment analyzer
	sentimentAnalyzer := sentiment.NewAnalyzer(cfg.QuantumGNNURL)
	fmt.Printf("✓ Quantum GNN analyzer connected to %s\n", cfg.QuantumGNNURL)

	// Content filter
	contentFilter := filter.NewContentFilter(
		cfg.BlockedKeywords,
		cfg.EnableMugshotDetection,
		cfg.EnableDefamationFilter,
	)
	fmt.Printf("✓ Content filter initialized\n")

	// Crawler
	crawlerInstance := crawler.NewCrawler(
		cfg.UserAgent,
		cfg.CrawlDelay,
		cfg.CrawlDepth,
		cfg.MaxConcurrent,
	)
	fmt.Printf("✓ Web crawler initialized\n")

	// Indexer
	idx, err := indexer.NewIndexer(cfg, sentimentAnalyzer, contentFilter)
	if err != nil {
		log.Fatalf("Failed to initialize indexer: %v", err)
	}
	defer idx.Close()

	count, _ := idx.GetIndexCount()
	fmt.Printf("✓ Search index loaded (%d documents)\n", count)
	fmt.Println()

	// Pipeline
	pipeline := indexer.NewPipeline(crawlerInstance, idx, *workers)

	// Process URLs
	if *url != "" {
		// Single URL mode
		fmt.Printf("📄 Indexing single URL: %s\n", *url)
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Println()

		if err := pipeline.ProcessURL(*url); err != nil {
			log.Printf("❌ Failed: %v", err)
		} else {
			fmt.Printf("✓ Successfully indexed\n")
		}

	} else if *urlFile != "" {
		// Batch mode from file
		urls, err := loadURLsFromFile(*urlFile)
		if err != nil {
			log.Fatalf("Failed to load URLs from file: %v", err)
		}

		fmt.Printf("📚 Indexing %d URLs from %s\n", len(urls), *urlFile)
		fmt.Printf("   Workers: %d\n", *workers)
		fmt.Printf("   Min sentiment: %.2f\n", *minSentiment)
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Println()

		result := pipeline.ProcessURLs(urls)

		fmt.Println()
		fmt.Println(result.Summary())

		// Show sample failures
		failCount := 0
		for _, outcome := range result.Outcomes {
			if outcome.Status != "indexed" && failCount < 5 {
				fmt.Printf("  Failed: %s - %s\n", outcome.URL, outcome.Error)
				failCount++
			}
		}
		if result.Failed > 5 {
			fmt.Printf("  ... and %d more failures\n", result.Failed-5)
		}

	} else {
		fmt.Println("❌ Please provide either --url or --urls flag")
		flag.Usage()
		os.Exit(1)
	}

	// Print final stats
	fmt.Println()
	fmt.Println("📊 Final Statistics:")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	stats := idx.GetStats()
	count, _ = idx.GetIndexCount()
	fmt.Printf("Total documents in index: %d\n", count)
	fmt.Printf("Processed this session:   %d\n", stats.TotalProcessed)
	fmt.Printf("Successfully indexed:     %d\n", stats.TotalIndexed)
	fmt.Printf("Filtered (total):         %d\n", stats.TotalFiltered)
	fmt.Printf("  - By content:           %d\n", stats.FilteredByContent)
	fmt.Printf("  - By sentiment:         %d\n", stats.FilteredBySentiment)
	fmt.Println()
}

// loadURLsFromFile loads URLs from a text file (one per line)
func loadURLsFromFile(filepath string) ([]string, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var urls []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		url := strings.TrimSpace(scanner.Text())
		// Skip empty lines and comments
		if url != "" && !strings.HasPrefix(url, "#") {
			urls = append(urls, url)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return urls, nil
}
