package searcher

import (
	"context"
	"log/slog"
	"sort"

	"github.com/ponchione/sirtopham/internal/codeintel"
)

// Searcher executes multi-query semantic search with deduplication,
// re-ranking, and optional call-graph hop expansion.
type Searcher struct {
	store    codeintel.Store
	embedder codeintel.Embedder
}

// New creates a Searcher from the given store and embedder.
func New(store codeintel.Store, embedder codeintel.Embedder) *Searcher {
	return &Searcher{store: store, embedder: embedder}
}

// Search embeds each query, runs vector search with the provided options,
// deduplicates by chunk ID, re-ranks by hit count with best-score tie
// breaking, optionally expands one hop, and returns up to MaxResults.
func (s *Searcher) Search(ctx context.Context, queries []string, opts codeintel.SearchOptions) ([]codeintel.SearchResult, error) {
	if len(queries) == 0 {
		return nil, nil
	}

	topK := opts.TopK
	if topK == 0 {
		topK = 10
	}

	type scored struct {
		result   codeintel.SearchResult
		hitCount int
		best     float64
	}
	seen := make(map[string]*scored)

	for _, q := range queries {
		vec, err := s.embedder.EmbedQuery(ctx, q)
		if err != nil {
			slog.Warn("embed query failed", "query", q, "error", err)
			continue
		}

		results, err := s.store.VectorSearch(ctx, vec, topK, opts.Filter)
		if err != nil {
			slog.Warn("vector search failed", "query", q, "error", err)
			continue
		}

		for _, r := range results {
			if existing, ok := seen[r.Chunk.ID]; ok {
				existing.hitCount++
				if r.Score > existing.best {
					existing.best = r.Score
					existing.result = r
				}
			} else {
				seen[r.Chunk.ID] = &scored{
					result:   r,
					hitCount: 1,
					best:     r.Score,
				}
			}
		}
	}

	directHits := make([]*scored, 0, len(seen))
	for _, s := range seen {
		directHits = append(directHits, s)
	}
	sort.Slice(directHits, func(i, j int) bool {
		if directHits[i].hitCount != directHits[j].hitCount {
			return directHits[i].hitCount > directHits[j].hitCount
		}
		return directHits[i].best > directHits[j].best
	})

	maxResults := opts.MaxResults
	if maxResults == 0 {
		maxResults = len(directHits)
	}

	var results []codeintel.SearchResult

	directBudget := len(directHits)
	hopBudget := 0
	if opts.EnableHopExpansion && opts.HopBudgetFraction > 0 {
		directBudget = min(int(float64(maxResults)*(1-opts.HopBudgetFraction)), len(directHits))
		hopBudget = maxResults - directBudget
	}

	seenIDs := make(map[string]bool)
	for i := 0; i < directBudget && i < len(directHits); i++ {
		h := directHits[i]
		h.result.HitCount = h.hitCount
		results = append(results, h.result)
		seenIDs[h.result.Chunk.ID] = true
	}

	if hopBudget > 0 {
		hops := s.expandHops(ctx, results, seenIDs, hopBudget)
		results = append(results, hops...)
	}

	if len(results) > maxResults {
		results = results[:maxResults]
	}

	return results, nil
}

// expandHops performs one-hop expansion through the call graph.
func (s *Searcher) expandHops(
	ctx context.Context,
	directHits []codeintel.SearchResult,
	seenIDs map[string]bool,
	budget int,
) []codeintel.SearchResult {
	var hops []codeintel.SearchResult

	for _, hit := range directHits {
		if len(hops) >= budget {
			break
		}

		allRefs := make([]codeintel.FuncRef, 0, len(hit.Chunk.Calls)+len(hit.Chunk.CalledBy))
		allRefs = append(allRefs, hit.Chunk.Calls...)
		allRefs = append(allRefs, hit.Chunk.CalledBy...)

		for _, ref := range allRefs {
			if len(hops) >= budget {
				break
			}
			chunks, err := s.store.GetByName(ctx, ref.Name)
			if err != nil {
				slog.Debug("hop lookup failed", "name", ref.Name, "error", err)
				continue
			}
			for _, c := range chunks {
				if len(hops) >= budget {
					break
				}
				if seenIDs[c.ID] {
					continue
				}
				seenIDs[c.ID] = true
				hops = append(hops, codeintel.SearchResult{
					Chunk:   c,
					Score:   0,
					FromHop: true,
				})
			}
		}
	}

	return hops
}
