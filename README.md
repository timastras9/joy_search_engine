# Joy Search Engine

A positive-content-only search engine that filters out defamatory content, negative news, and mugshots using quantum GNN sentiment analysis.

## Architecture

- **Web Crawler**: Scrapes and collects web content
- **Quantum GNN Sentiment Filter**: Uses quantum graph neural network for sentiment analysis
- **Content Filters**: Removes mugshots, defamatory content, negative news
- **Search Index**: Bleve-based full-text search index
- **API Server**: REST API for searching positive content

## Components

### 1. Crawler (`cmd/crawler/`)
Fetches web pages and extracts content

### 2. Sentiment Analyzer (`pkg/sentiment/`)
Interfaces with quantum GNN MCP server for sentiment analysis

### 3. Content Filter (`pkg/filter/`)
Applies additional filters (mugshots, defamation detection)

### 4. Indexer (`pkg/indexer/`)
Indexes positive content into searchable database

### 5. Search API (`cmd/server/`)
Serves search queries

## Storage Requirements

- Quantum GNN cache: ~100-500 GB
- Search index: ~500 GB - 5 TB (depending on scale)
- Total: ~1-6 TB recommended

## Quick Start

### 1. Start Quantum GNN MCP Server

First, start the quantum sentiment GNN server:

```bash
cd ../quantum-sentiment-gnn
go run cmd/serve-mcp/main.go --port 8081 --model models/quantum_sentiment_gnn.json
```

### 2. Index Content

Index URLs from a file (filters applied automatically):

```bash
# Index from URL file
go run cmd/indexer/main.go --urls example_urls.txt --workers 5

# Index a single URL
go run cmd/indexer/main.go --url https://www.goodnewsnetwork.org/

# Custom settings
go run cmd/indexer/main.go \
  --urls urls.txt \
  --workers 10 \
  --min-sentiment 0.7 \
  --quantum-url http://localhost:8081 \
  --index ./data/index
```

### 3. Start Search API Server

```bash
go run cmd/server/main.go --port 8080
```

### 4. Search for Positive Content

```bash
# Search query
curl http://localhost:8080/search?q=happy+news&page=0&size=10

# Get document by URL
curl http://localhost:8080/document?url=https://example.com/article
```

## How It Works

1. **Crawler** fetches web pages and extracts content
2. **Content Filter** removes mugshots, defamatory content, blocked keywords
3. **Quantum GNN** analyzes sentiment via MCP server (text → graph → quantum circuits → score)
4. **Filter by Score**: Only content with sentiment ≥ 0.6 (configurable) is indexed
5. **Bleve Index** stores positive content for fast full-text search
6. **Search API** serves queries with results sorted by sentiment and relevance
