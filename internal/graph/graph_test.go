package graph

import (
	"strings"
	"testing"
)

func TestGraphAddEdge(t *testing.T) {
	g := New()
	g.AddEdge("frontend", "api")
	g.AddEdge("api", "redis")
	g.AddEdge("frontend", "api")

	edges := g.Edges()
	if len(edges) != 2 {
		t.Fatalf("expected 2 edges, got %d", len(edges))
	}

	for _, e := range edges {
		if e.Source == "frontend" && e.Destination == "api" {
			if e.Count != 2 {
				t.Fatalf("expected count 2, got %d", e.Count)
			}
		}
	}
}

func TestGraphMermaid(t *testing.T) {
	g := New()
	g.AddEdge("frontend", "api")
	g.AddEdge("api", "redis")

	m := g.Mermaid()
	if !strings.Contains(m, "graph LR") {
		t.Fatal("expected graph LR header")
	}
	if !strings.Contains(m, "frontend") {
		t.Fatal("expected frontend node")
	}
	if !strings.Contains(m, "api") {
		t.Fatal("expected api node")
	}
	if !strings.Contains(m, "redis") {
		t.Fatal("expected redis node")
	}
	if !strings.Contains(m, "-->") {
		t.Fatal("expected edge arrows")
	}
}

func TestGraphEmpty(t *testing.T) {
	g := New()
	m := g.Mermaid()
	if !strings.Contains(m, "(no edges)") {
		t.Fatal("expected no edges message")
	}
}

func TestSanitizeNodeID(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"frontend", "frontend"},
		{"api-v2", "api-v2"},
		{"10.0.1.5", "n10_0_1_5"},
		{"my_service", "my_service"},
		{"", "node"},
		{"test:app", "test_app"},
	}
	for _, c := range cases {
		result := sanitizeNodeID(c.input)
		if result != c.expected {
			t.Errorf("sanitizeNodeID(%q) = %q, want %q", c.input, result, c.expected)
		}
	}
}

func TestASCII(t *testing.T) {
	g := New()
	g.AddEdge("frontend", "api")
	g.AddEdge("api", "redis")

	s := g.ASCII()
	if !strings.Contains(s, "frontend ──→ api") {
		t.Fatalf("expected frontend ──→ api, got %s", s)
	}
	if !strings.Contains(s, "api ──→ redis") {
		t.Fatalf("expected api ──→ redis, got %s", s)
	}
}

func TestASCIIEmpty(t *testing.T) {
	g := New()
	s := g.ASCII()
	if s != "(no edges)" {
		t.Fatalf("expected (no edges), got %s", s)
	}
}

func TestASCIIMultiOut(t *testing.T) {
	g := New()
	g.AddEdge("api", "redis")
	g.AddEdge("api", "postgres")

	s := g.ASCII()
	if !strings.Contains(s, "├─→ postgres") {
		t.Fatalf("expected ├─→ postgres, got %s", s)
	}
	if !strings.Contains(s, "└─→ redis") {
		t.Fatalf("expected └─→ redis, got %s", s)
	}
}

func TestASCIIWithCount(t *testing.T) {
	g := New()
	g.AddEdge("a", "b")
	g.AddEdge("a", "b")

	s := g.ASCII()
	if !strings.Contains(s, "(2 connections)") {
		t.Fatalf("expected connection count, got %s", s)
	}
}

func TestStringDelegatesToASCII(t *testing.T) {
	g := New()
	g.AddEdge("x", "y")
	s := g.String()
	if !strings.Contains(s, "x ──→ y") {
		t.Fatal("String() should delegate to ASCII()")
	}
}
