package engine

import (
	"context"
	"fmt"
	"time"
)

// StepEntry holds a step with its ID for ordered execution.
type StepEntry struct {
	ID   string
	Step Step
}

type Pipeline struct {
	name       string
	date       time.Time
	collectors map[string]Collector
	steps      []StepEntry
}

func NewPipeline(name string) *Pipeline {
	return &Pipeline{
		name:       name,
		date:       time.Now().UTC(),
		collectors: make(map[string]Collector),
		steps:      nil,
	}
}

func (p *Pipeline) AddCollector(id string, collector Collector) error {
	if _, ok := p.collectors[id]; ok {
		return fmt.Errorf("collector %s already exists", id)
	}

	p.collectors[id] = collector
	return nil
}

func (p *Pipeline) AddStep(id string, step Step) error {
	for _, entry := range p.steps {
		if entry.ID == id {
			return fmt.Errorf("step %s already exists", id)
		}
	}

	p.steps = append(p.steps, StepEntry{ID: id, Step: step})
	return nil
}

func (p *Pipeline) Date() time.Time {
	return p.date
}

func (p *Pipeline) Collectors() map[string]Collector {
	return p.collectors
}

func (p *Pipeline) Steps() []StepEntry {
	return p.steps
}

func (p *Pipeline) GetCollector(id string) (Collector, bool) {
	collector, ok := p.collectors[id]
	if !ok {
		return nil, false
	}
	return collector, true
}

func (p *Pipeline) Run(ctx context.Context) (map[string]Result, error) {
	results := make(map[string]Result)

	for _, entry := range p.steps {
		// Check context cancellation before each step
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("context cancelled while running pipeline at step '%s': %w", entry.ID, err)
		}

		result, err := entry.Step.Resolve(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve step '%s': %w", entry.ID, err)
		}

		result.ID = entry.ID

		results[entry.ID] = result
	}

	return results, nil
}
