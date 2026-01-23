package docsearch

import (
	"strings"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/search/query"
)

// SearchResult represents a search hit with score.
type SearchResult struct {
	Chunk DocChunk
	Score float64
}

// Searcher provides full-text search over documentation.
type Searcher struct {
	index  bleve.Index
	chunks map[string]DocChunk // ID -> chunk for retrieval
}

// indexDoc is the structure indexed by Bleve.
type indexDoc struct {
	Topic   string `json:"topic"`
	Heading string `json:"heading"`
	Content string `json:"content"`
}

// NewSearcher creates a new searcher with an in-memory Bleve index.
func NewSearcher(chunks []DocChunk) (*Searcher, error) {
	// Build chunk lookup map
	chunkMap := make(map[string]DocChunk, len(chunks))
	for _, chunk := range chunks {
		chunkMap[chunk.ID] = chunk
	}

	// Create in-memory index with default mapping (uses BM25 by default)
	mapping := bleve.NewIndexMapping()
	index, err := bleve.NewMemOnly(mapping)
	if err != nil {
		return nil, err
	}

	// Index each chunk
	for _, chunk := range chunks {
		doc := indexDoc{
			Topic:   chunk.Topic,
			Heading: chunk.Heading,
			Content: chunk.Content,
		}
		if err := index.Index(chunk.ID, doc); err != nil {
			index.Close()
			return nil, err
		}
	}

	return &Searcher{
		index:  index,
		chunks: chunkMap,
	}, nil
}

// Search performs a search with optional fuzzy matching.
// When exact is false, fuzzy matching is enabled for typo tolerance.
func (s *Searcher) Search(queryStr string, limit int, exact bool) ([]SearchResult, error) {
	var q query.Query

	if exact {
		// Use match query for exact matching (still uses BM25 scoring)
		q = bleve.NewMatchQuery(queryStr)
	} else {
		// Build a boolean query that combines exact and fuzzy matches
		// This gives us both precision and typo tolerance
		boolQuery := bleve.NewBooleanQuery()

		// Add exact match with high boost
		matchQuery := bleve.NewMatchQuery(queryStr)
		matchQuery.SetBoost(2.0)
		boolQuery.AddShould(matchQuery)

		// Add fuzzy matches for each term
		terms := strings.Fields(queryStr)
		for _, term := range terms {
			if len(term) >= 4 { // Only fuzzy match longer terms
				fuzzyQuery := bleve.NewFuzzyQuery(term)
				fuzzyQuery.SetFuzziness(getFuzziness(term))
				boolQuery.AddShould(fuzzyQuery)
			}
		}

		// Also try phrase matching for multi-word queries
		if len(terms) > 1 {
			phraseQuery := bleve.NewMatchPhraseQuery(queryStr)
			phraseQuery.SetBoost(1.5)
			boolQuery.AddShould(phraseQuery)
		}

		q = boolQuery
	}

	// Execute search
	searchRequest := bleve.NewSearchRequest(q)
	searchRequest.Size = limit

	result, err := s.index.Search(searchRequest)
	if err != nil {
		return nil, err
	}

	// Convert to SearchResults
	results := make([]SearchResult, 0, len(result.Hits))
	for _, hit := range result.Hits {
		if chunk, ok := s.chunks[hit.ID]; ok {
			results = append(results, SearchResult{
				Chunk: chunk,
				Score: hit.Score,
			})
		}
	}

	return results, nil
}

// getFuzziness returns the appropriate fuzziness level based on term length.
// Shorter words get less tolerance, longer words get more.
func getFuzziness(term string) int {
	switch {
	case len(term) <= 4:
		return 1
	case len(term) <= 7:
		return 1
	default:
		return 2
	}
}

// Close releases resources held by the searcher.
func (s *Searcher) Close() error {
	return s.index.Close()
}
