package export

import (
	"time"

	"github.com/moyashiwithdevice/kubectl-detective/internal/graph"
)

type Report struct {
	Timestamp time.Time
	Duration  time.Duration
	Graph     *graph.Graph
}

func NewReport(g *graph.Graph, duration time.Duration) *Report {
	return &Report{
		Timestamp: time.Now(),
		Duration:  duration,
		Graph:     g,
	}
}

func (r *Report) FlowCount() int {
	n := 0
	for _, e := range r.Graph.Edges() {
		n += e.Count
	}
	return n
}
