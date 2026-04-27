package indexer

import (
	"context"
	"log/slog"

	"github.com/ponchione/sodoryard/internal/codeintel"
)

type describeEmbedOptions struct {
	FailOnDescribe bool
	RecordFailures bool
	LogMessage     string
	OnSuccess      func(*parsedFile)
}

type describeEmbedResult struct {
	chunkCounts  map[string]int
	failed       []string
	filesIndexed int
	totalChunks  int
}

func describeEmbedStoreFiles(
	ctx context.Context,
	parsed []parsedFile,
	store codeintel.Store,
	embedder codeintel.Embedder,
	describer codeintel.Describer,
	opts describeEmbedOptions,
) describeEmbedResult {
	result := describeEmbedResult{
		chunkCounts: make(map[string]int, len(parsed)),
		failed:      make([]string, 0),
	}

	for i := range parsed {
		pf := &parsed[i]

		descriptions, err := describeParsedFile(ctx, pf, describer)
		if err != nil {
			slog.Warn("describe failed", "path", pf.relPath, "err", err)
			if opts.RecordFailures {
				result.failed = append(result.failed, pf.relPath)
			}
			if opts.FailOnDescribe {
				continue
			}
			descriptions = nil
		}

		embedTexts := applyDescriptions(pf.chunks, descriptions)
		embeddings, err := embedder.EmbedTexts(ctx, embedTexts)
		if err != nil {
			slog.Warn("embed failed", "path", pf.relPath, "err", err)
			if opts.RecordFailures {
				result.failed = append(result.failed, pf.relPath)
			}
			continue
		}
		for j := range pf.chunks {
			if j < len(embeddings) {
				pf.chunks[j].Embedding = embeddings[j]
			}
		}

		if err := store.Upsert(ctx, pf.chunks); err != nil {
			slog.Warn("upsert failed", "path", pf.relPath, "err", err)
			if opts.RecordFailures {
				result.failed = append(result.failed, pf.relPath)
			}
			continue
		}

		if opts.OnSuccess != nil {
			opts.OnSuccess(pf)
		}
		result.filesIndexed++
		result.totalChunks += len(pf.chunks)
		result.chunkCounts[pf.relPath] = len(pf.chunks)
		slog.Info(opts.LogMessage, "path", pf.relPath, "chunks", len(pf.chunks))
	}

	return result
}

func describeParsedFile(ctx context.Context, pf *parsedFile, describer codeintel.Describer) ([]codeintel.Description, error) {
	descContent := string(pf.content)
	if relCtx := formatRelationshipContext(pf.chunks); relCtx != "" {
		descContent = descContent + "\n\n" + relCtx
	}
	return describer.DescribeFile(ctx, descContent, "")
}

func applyDescriptions(chunks []codeintel.Chunk, descriptions []codeintel.Description) []string {
	descMap := make(map[string]string, len(descriptions))
	for _, d := range descriptions {
		descMap[d.Name] = d.Description
	}

	embedTexts := make([]string, len(chunks))
	for j := range chunks {
		desc := descMap[chunks[j].Name]
		chunks[j].Description = desc
		embedTexts[j] = chunks[j].Signature + "\n" + desc
	}
	return embedTexts
}
