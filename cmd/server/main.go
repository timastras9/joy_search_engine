package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/timastras9/joy_search_engine/internal/config"
	"github.com/timastras9/joy_search_engine/pkg/filter"
	"github.com/timastras9/joy_search_engine/pkg/indexer"
	"github.com/timastras9/joy_search_engine/pkg/models"
	"github.com/timastras9/joy_search_engine/pkg/sentiment"
)

type Server struct {
	indexer *indexer.Indexer
	config  *config.Config
}

func main() {
	// Parse flags
	port := flag.String("port", "8080", "Server port")
	indexPath := flag.String("index", "./data/index", "Path to search index")
	quantumURL := flag.String("quantum-url", "http://localhost:8081", "Quantum GNN MCP server URL")
	flag.Parse()

	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║           Joy Search Engine - API Server                    ║")
	fmt.Println("║          Only Positive Content, No Negativity                ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Load config
	cfg := config.Default()
	cfg.IndexPath = *indexPath
	cfg.QuantumGNNURL = *quantumURL
	cfg.ServerPort = *port

	// Initialize components
	fmt.Println("🔧 Initializing components...")

	sentimentAnalyzer := sentiment.NewAnalyzer(cfg.QuantumGNNURL)
	fmt.Printf("✓ Quantum GNN analyzer connected to %s\n", cfg.QuantumGNNURL)

	contentFilter := filter.NewContentFilter(
		cfg.BlockedKeywords,
		cfg.EnableMugshotDetection,
		cfg.EnableDefamationFilter,
	)
	fmt.Printf("✓ Content filter initialized\n")

	idx, err := indexer.NewIndexer(cfg, sentimentAnalyzer, contentFilter)
	if err != nil {
		log.Fatalf("Failed to initialize indexer: %v", err)
	}
	defer idx.Close()

	count, _ := idx.GetIndexCount()
	fmt.Printf("✓ Search index loaded (%d documents)\n", count)
	fmt.Println()

	// Create server
	server := &Server{
		indexer: idx,
		config:  cfg,
	}

	// Setup routes
	router := mux.NewRouter()

	// API routes
	router.HandleFunc("/api/search", server.handleSearch).Methods("GET")
	router.HandleFunc("/api/document", server.handleGetDocument).Methods("GET")
	router.HandleFunc("/api/stats", server.handleStats).Methods("GET")

	// Serve static files (web interface)
	router.PathPrefix("/").Handler(http.FileServer(http.Dir("./web")))

	// Start server
	addr := ":" + cfg.ServerPort
	fmt.Printf("🚀 Server starting on http://localhost%s\n", addr)
	fmt.Printf("📊 Stats: http://localhost%s/api/stats\n", addr)
	fmt.Printf("🔍 Search: http://localhost%s/api/search?q=your+query\n", addr)
	fmt.Printf("🌐 Web UI: http://localhost%s\n", addr)
	fmt.Println()

	if err := http.ListenAndServe(addr, corsMiddleware(router)); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

// handleSearch handles search queries
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, "Missing query parameter 'q'", http.StatusBadRequest)
		return
	}

	// Parse pagination
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 0 {
		page = 0
	}

	size, _ := strconv.Atoi(r.URL.Query().Get("size"))
	if size <= 0 || size > 100 {
		size = 10
	}

	// Execute search
	result, err := s.indexer.Search(query, page, size)
	if err != nil {
		http.Error(w, fmt.Sprintf("Search failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Return JSON
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// handleGetDocument retrieves a specific document by URL
func (s *Server) handleGetDocument(w http.ResponseWriter, r *http.Request) {
	url := r.URL.Query().Get("url")
	if url == "" {
		http.Error(w, "Missing query parameter 'url'", http.StatusBadRequest)
		return
	}

	doc, err := s.indexer.SearchByURL(url)
	if err != nil {
		http.Error(w, fmt.Sprintf("Document not found: %v", err), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(doc)
}

// handleStats returns index statistics
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := s.indexer.GetStats()
	count, _ := s.indexer.GetIndexCount()

	response := map[string]interface{}{
		"total_documents":      count,
		"total_processed":      stats.TotalProcessed,
		"total_indexed":        stats.TotalIndexed,
		"total_filtered":       stats.TotalFiltered,
		"filtered_by_content":  stats.FilteredByContent,
		"filtered_by_sentiment": stats.FilteredBySentiment,
		"last_updated":         stats.LastUpdated,
		"min_sentiment_score":  s.config.MinSentimentScore,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// corsMiddleware adds CORS headers
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
