package server_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/ponchione/sodoryard/internal/brain"
	"github.com/ponchione/sodoryard/internal/chain"
	"github.com/ponchione/sodoryard/internal/config"
	appdb "github.com/ponchione/sodoryard/internal/db"
	"github.com/ponchione/sodoryard/internal/operator"
	rtpkg "github.com/ponchione/sodoryard/internal/runtime"
	"github.com/ponchione/sodoryard/internal/server"
)

type chainTestBrain struct {
	docs map[string]string
}

func (b *chainTestBrain) ReadDocument(_ context.Context, path string) (string, error) {
	content, ok := b.docs[path]
	if !ok {
		return "", fmt.Errorf("missing document %s", path)
	}
	return content, nil
}

func (b *chainTestBrain) WriteDocument(_ context.Context, path string, content string) error {
	b.docs[path] = content
	return nil
}

func (b *chainTestBrain) PatchDocument(context.Context, string, string, string) error {
	return nil
}

func (b *chainTestBrain) SearchKeyword(context.Context, string) ([]brain.SearchHit, error) {
	return nil, nil
}

func (b *chainTestBrain) ListDocuments(context.Context, string) ([]string, error) {
	return nil, nil
}

func TestChainInspectorEndpoints(t *testing.T) {
	ctx := context.Background()
	db := newChainInspectorTestDB(t)
	store := chain.NewStore(db)
	chainID, err := store.StartChain(ctx, chain.ChainSpec{ChainID: "chain-web", SourceTask: "inspect"})
	if err != nil {
		t.Fatalf("StartChain returned error: %v", err)
	}
	stepID, err := store.StartStep(ctx, chain.StepSpec{ChainID: chainID, SequenceNum: 1, Role: "coder", Task: "code"})
	if err != nil {
		t.Fatalf("StartStep returned error: %v", err)
	}
	receiptPath := "receipts/coder/chain-web-step-001.md"
	if err := store.CompleteStep(ctx, chain.CompleteStepParams{StepID: stepID, Status: "completed", Verdict: "accepted", ReceiptPath: receiptPath, TokensUsed: 42}); err != nil {
		t.Fatalf("CompleteStep returned error: %v", err)
	}
	if err := store.CompleteChain(ctx, chainID, "completed", "done"); err != nil {
		t.Fatalf("CompleteChain returned error: %v", err)
	}

	cfg := &config.Config{
		ProjectRoot: t.TempDir(),
		Routing: config.RoutingConfig{
			Default: config.RouteConfig{Provider: "codex", Model: "test-model"},
		},
	}
	opSvc, err := operator.NewForRuntime(&rtpkg.OrchestratorRuntime{
		Config:       cfg,
		Database:     db,
		ChainStore:   store,
		BrainBackend: &chainTestBrain{docs: map[string]string{receiptPath: "receipt content"}},
		Cleanup:      func() {},
	}, operator.Options{})
	if err != nil {
		t.Fatalf("NewForRuntime returned error: %v", err)
	}
	t.Cleanup(opSvc.Close)

	srv := server.New(server.Config{Host: "127.0.0.1", Port: 0}, newTestLogger())
	server.NewChainInspectorHandler(srv, opSvc, newTestLogger())
	_, base := startServer(t, srv)

	var chains []struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	getJSON(t, base+"/api/chains", &chains)
	if len(chains) != 1 || chains[0].ID != chainID || chains[0].Status != "completed" {
		t.Fatalf("chains response = %+v, want completed chain-web", chains)
	}

	var detail struct {
		Chain struct {
			ID string `json:"id"`
		} `json:"chain"`
		Steps []struct {
			Role        string `json:"role"`
			ReceiptPath string `json:"receipt_path"`
		} `json:"steps"`
		Receipts []struct {
			Step string `json:"step"`
			Path string `json:"path"`
		} `json:"receipts"`
	}
	getJSON(t, base+"/api/chains/"+chainID, &detail)
	if detail.Chain.ID != chainID || len(detail.Steps) != 1 || detail.Steps[0].Role != "coder" {
		t.Fatalf("detail response = %+v, want chain detail", detail)
	}
	if len(detail.Receipts) != 1 || detail.Receipts[0].Path != receiptPath {
		t.Fatalf("receipts = %+v, want step receipt", detail.Receipts)
	}

	var receipt struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	getJSON(t, base+"/api/chains/"+chainID+"/receipt?step=1", &receipt)
	if receipt.Path != receiptPath || receipt.Content != "receipt content" {
		t.Fatalf("receipt = %+v, want content", receipt)
	}
}

func newChainInspectorTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := appdb.OpenDB(context.Background(), filepath.Join(t.TempDir(), "server-chains.db"))
	if err != nil {
		t.Fatalf("OpenDB returned error: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := appdb.InitIfNeeded(context.Background(), db); err != nil {
		t.Fatalf("InitIfNeeded returned error: %v", err)
	}
	if err := appdb.EnsureChainSchema(context.Background(), db); err != nil {
		t.Fatalf("EnsureChainSchema returned error: %v", err)
	}
	return db
}

func getJSON(t *testing.T, url string, v any) {
	t.Helper()
	client := http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("GET %s failed: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s status = %d, want 200", url, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatalf("decode %s: %v", url, err)
	}
}
