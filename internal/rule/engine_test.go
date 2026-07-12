package rule

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func writeTempWASM(t *testing.T, data []byte) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "test.wasm")
	if err := os.WriteFile(p, data, 0644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestBuildSimpleTestWASM(t *testing.T) {
	wasm := BuildSimpleTestWASM("test-rule", "A test rule")
	if len(wasm) == 0 {
		t.Fatal("empty wasm")
	}
	if wasm[0] != 0x00 || wasm[1] != 0x61 || wasm[2] != 0x73 || wasm[3] != 0x6d {
		t.Fatalf("invalid magic: %x", wasm[:4])
	}
}

func TestEngineLoadAndEvaluate(t *testing.T) {
	wasm := BuildSimpleTestWASM("test-rule", "A test rule for validation")
	path := writeTempWASM(t, wasm)

	ctx := context.Background()
	eng := NewEngine()
	defer eng.Close(ctx)

	if err := eng.Load(ctx, path); err != nil {
		t.Fatalf("load: %v", err)
	}

	if eng.Name() != "test-rule" {
		t.Fatalf("name: got %q, want %q", eng.Name(), "test-rule")
	}
	if eng.Description() != "A test rule for validation" {
		t.Fatalf("desc: got %q, want %q", eng.Description(), "A test rule for validation")
	}
}

func TestEngineEvaluatePass(t *testing.T) {
	wasm := BuildSimpleTestWASM("pass-rule", "Always passes")
	path := writeTempWASM(t, wasm)

	ctx := context.Background()
	eng := NewEngine()
	defer eng.Close(ctx)

	if err := eng.Load(ctx, path); err != nil {
		t.Fatalf("load: %v", err)
	}

	result, err := eng.Evaluate(ctx, []byte(`{"flows":[]}`))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if result != Pass {
		t.Fatalf("expected Pass, got %d", result)
	}
}

func TestEngineEvaluateAlert(t *testing.T) {
	wasm := BuildEvalTestWASM("alert-rule", "Always alerts", true)
	path := writeTempWASM(t, wasm)

	ctx := context.Background()
	eng := NewEngine()
	defer eng.Close(ctx)

	if err := eng.Load(ctx, path); err != nil {
		t.Fatalf("load: %v", err)
	}

	result, err := eng.Evaluate(ctx, []byte(`{"flows":[{"src_ip":"10.0.0.1"}]}`))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if result != Alert {
		t.Fatalf("expected Alert, got %d", result)
	}
}

func TestSerializeContext(t *testing.T) {
	ctx := &Context{
		Throughputs: []ThroughputEntry{
			{SrcIP: "10.0.0.1", DstIP: "10.0.0.2", TxBytes: 1024, RxBytes: 2048},
		},
		Retrans: []RetransEntry{
			{SrcIP: "10.0.0.1", DstIP: "10.0.0.2", Count: 5},
		},
		Flows: []FlowEntry{
			{SrcIP: "10.0.0.1", DstIP: "10.0.0.2", SrcPort: 8080, DstPort: 3306},
		},
	}

	data, err := SerializeContext(ctx)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("empty serialized data")
	}

	var decoded Context
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(decoded.Throughputs) != 1 {
		t.Fatalf("expected 1 throughput, got %d", len(decoded.Throughputs))
	}
	if decoded.Throughputs[0].TxBytes != 1024 {
		t.Fatalf("expected TxBytes 1024, got %d", decoded.Throughputs[0].TxBytes)
	}
	if len(decoded.Retrans) != 1 {
		t.Fatalf("expected 1 retrans, got %d", len(decoded.Retrans))
	}
	if len(decoded.Flows) != 1 {
		t.Fatalf("expected 1 flow, got %d", len(decoded.Flows))
	}
}

func TestMergeContexts(t *testing.T) {
	a := &Context{
		Throughputs: []ThroughputEntry{{SrcIP: "1.1.1.1"}},
	}
	b := &Context{
		Retrans: []RetransEntry{{SrcIP: "2.2.2.2"}},
	}

	merged := MergeContexts(a, b)
	if len(merged.Throughputs) != 1 {
		t.Fatalf("expected 1 throughput, got %d", len(merged.Throughputs))
	}
	if len(merged.Retrans) != 1 {
		t.Fatalf("expected 1 retrans, got %d", len(merged.Retrans))
	}
}

func TestMergeContextsNil(t *testing.T) {
	a := &Context{Flows: []FlowEntry{{SrcIP: "1.1.1.1"}}}

	if MergeContexts(nil, a) != a {
		t.Fatal("MergeContexts(nil, a) should return a")
	}
	if MergeContexts(a, nil) != a {
		t.Fatal("MergeContexts(a, nil) should return a")
	}
	if MergeContexts(nil, nil) != nil {
		t.Fatal("MergeContexts(nil, nil) should return nil")
	}
}

func TestEngineInvalidWASM(t *testing.T) {
	path := writeTempWASM(t, []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00})

	ctx := context.Background()
	eng := NewEngine()
	defer eng.Close(ctx)

	err := eng.Load(ctx, path)
	if err == nil {
		t.Fatal("expected error for invalid wasm")
	}
}

func TestEngineMissingFile(t *testing.T) {
	ctx := context.Background()
	eng := NewEngine()
	defer eng.Close(ctx)

	err := eng.Load(ctx, "/nonexistent/file.wasm")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}
