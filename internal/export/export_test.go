package export

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/moyashiwithdevice/kubectl-detective/internal/graph"
)

func testReport() *Report {
	g := graph.New()
	g.AddEdge("frontend", "api")
	g.AddEdge("api", "redis")
	g.AddEdge("frontend", "api")
	return NewReport(g, 10*time.Second)
}

func TestWriteCSV(t *testing.T) {
	r := testReport()
	var buf bytes.Buffer
	if err := WriteCSV(&buf, r); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "source,destination,connections") {
		t.Fatal("missing CSV header")
	}
	if !strings.Contains(out, "frontend,api,2") {
		t.Fatalf("expected frontend,api,2, got:\n%s", out)
	}
	if !strings.Contains(out, "api,redis,1") {
		t.Fatalf("expected api,redis,1, got:\n%s", out)
	}
}

func TestFormatCSV(t *testing.T) {
	r := testReport()
	out := FormatCSV(r)
	if !strings.HasPrefix(out, "source,") {
		t.Fatal("CSV should start with header")
	}
}

func TestWriteJSON(t *testing.T) {
	r := testReport()
	var buf bytes.Buffer
	if err := WriteJSON(&buf, r); err != nil {
		t.Fatal(err)
	}

	var jr jsonReport
	if err := json.Unmarshal(buf.Bytes(), &jr); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if jr.FlowCount != 3 {
		t.Fatalf("expected flow_count 3, got %d", jr.FlowCount)
	}
	if len(jr.Edges) != 2 {
		t.Fatalf("expected 2 edges, got %d", len(jr.Edges))
	}
	if jr.Timestamp == "" {
		t.Fatal("timestamp should not be empty")
	}
	if jr.Duration != "10s" {
		t.Fatalf("expected duration 10s, got %s", jr.Duration)
	}
}

func TestFormatJSON(t *testing.T) {
	r := testReport()
	out := FormatJSON(r)
	if !strings.Contains(out, "\"flow_count\": 3") {
		t.Fatal("JSON should contain flow_count")
	}
}

func TestWriteHTML(t *testing.T) {
	r := testReport()
	var buf bytes.Buffer
	if err := WriteHTML(&buf, r); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "<!DOCTYPE html>") {
		t.Fatal("HTML should contain doctype")
	}
	if !strings.Contains(out, "frontend") {
		t.Fatal("HTML should contain node names")
	}
	if !strings.Contains(out, "graph LR") {
		t.Fatal("HTML should contain Mermaid block")
	}
	if !strings.Contains(out, "Total flows: 3") {
		t.Fatal("HTML should contain flow count")
	}
}

func TestFormatHTML(t *testing.T) {
	r := testReport()
	out := FormatHTML(r)
	if !strings.Contains(out, "<table>") {
		t.Fatal("HTML should contain table")
	}
}

func TestReportFlowCount(t *testing.T) {
	g := graph.New()
	g.AddEdge("a", "b")
	g.AddEdge("a", "b")
	g.AddEdge("a", "b")
	r := NewReport(g, 5*time.Second)
	if r.FlowCount() != 3 {
		t.Fatalf("expected 3, got %d", r.FlowCount())
	}
}

func TestCSVEmpty(t *testing.T) {
	g := graph.New()
	r := NewReport(g, 0)
	var buf bytes.Buffer
	if err := WriteCSV(&buf, r); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "source,destination,connections") {
		t.Fatal("CSV should have header even when empty")
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line (header only), got %d", len(lines))
	}
}

func TestJSONEmpty(t *testing.T) {
	g := graph.New()
	r := NewReport(g, 0)
	var buf bytes.Buffer
	if err := WriteJSON(&buf, r); err != nil {
		t.Fatal(err)
	}
	var jr jsonReport
	if err := json.Unmarshal(buf.Bytes(), &jr); err != nil {
		t.Fatal(err)
	}
	if jr.FlowCount != 0 {
		t.Fatal("expected 0 flows")
	}
	if len(jr.Edges) != 0 {
		t.Fatal("expected 0 edges")
	}
}

func TestHTMLEmpty(t *testing.T) {
	g := graph.New()
	r := NewReport(g, 0)
	out := FormatHTML(r)
	if !strings.Contains(out, "(no edges)") {
		t.Fatal("empty HTML should contain no edges message")
	}
}
