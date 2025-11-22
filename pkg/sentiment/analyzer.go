package sentiment

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Analyzer interfaces with the Quantum GNN MCP server for sentiment analysis
type Analyzer struct {
	mcpURL     string
	httpClient *http.Client
}

// NewAnalyzer creates a new sentiment analyzer
func NewAnalyzer(mcpURL string) *Analyzer {
	return &Analyzer{
		mcpURL: mcpURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// TextGraph represents text as a graph for quantum GNN processing
type TextGraph struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

// Node represents a word/phrase in the text graph
type Node struct {
	ID       string            `json:"id"`
	Text     string            `json:"text"`
	Features map[string]float64 `json:"features"`
}

// Edge represents semantic relationship between nodes
type Edge struct {
	Source string  `json:"source"`
	Target string  `json:"target"`
	Weight float64 `json:"weight"`
}

// SentimentRequest sent to quantum GNN MCP server
type SentimentRequest struct {
	Graph      TextGraph `json:"graph"`
	TaskType   string    `json:"task_type"` // "sentiment_analysis"
	Parameters map[string]interface{} `json:"parameters,omitempty"`
}

// SentimentResponse from quantum GNN MCP server
type SentimentResponse struct {
	Score      float64            `json:"score"`       // 0.0 (negative) to 1.0 (positive)
	Confidence float64            `json:"confidence"`  // 0.0 to 1.0
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// AnalyzeText converts text to graph and analyzes sentiment using quantum GNN
func (a *Analyzer) AnalyzeText(text string) (*SentimentResponse, error) {
	// Convert text to graph representation
	graph := a.textToGraph(text)

	// Prepare request
	reqBody := SentimentRequest{
		Graph:    graph,
		TaskType: "sentiment_analysis",
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Call quantum GNN MCP server
	resp, err := a.httpClient.Post(
		a.mcpURL+"/quantum/analyze",
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to call quantum GNN: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("quantum GNN returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var result SentimentResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// textToGraph converts text into a graph representation for quantum GNN
// Creates nodes from sentences/phrases and edges based on semantic relationships
func (a *Analyzer) textToGraph(text string) TextGraph {
	// Simple implementation: sentences as nodes, sequential edges
	sentences := strings.Split(text, ". ")

	nodes := make([]Node, 0, len(sentences))
	edges := make([]Edge, 0, len(sentences)-1)

	for i, sentence := range sentences {
		sentence = strings.TrimSpace(sentence)
		if sentence == "" {
			continue
		}

		// Create node for each sentence
		nodeID := fmt.Sprintf("node_%d", i)
		nodes = append(nodes, Node{
			ID:   nodeID,
			Text: sentence,
			Features: map[string]float64{
				"length":     float64(len(sentence)),
				"word_count": float64(len(strings.Fields(sentence))),
				"position":   float64(i) / float64(len(sentences)),
			},
		})

		// Create edge to previous sentence (sequential relationship)
		if i > 0 {
			edges = append(edges, Edge{
				Source: fmt.Sprintf("node_%d", i-1),
				Target: nodeID,
				Weight: 1.0,
			})
		}
	}

	return TextGraph{
		Nodes: nodes,
		Edges: edges,
	}
}

// AnalyzeBatch analyzes multiple texts in batch for efficiency
func (a *Analyzer) AnalyzeBatch(texts []string) ([]*SentimentResponse, error) {
	results := make([]*SentimentResponse, len(texts))

	for i, text := range texts {
		result, err := a.AnalyzeText(text)
		if err != nil {
			return nil, fmt.Errorf("failed to analyze text %d: %w", i, err)
		}
		results[i] = result
	}

	return results, nil
}
