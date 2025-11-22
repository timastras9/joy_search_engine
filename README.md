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

## Usage

```bash
# Start the search engine
go run cmd/server/main.go

# Run the crawler
go run cmd/crawler/main.go --urls urls.txt

# Query the search
curl http://localhost:8080/search?q=happy+news
```
