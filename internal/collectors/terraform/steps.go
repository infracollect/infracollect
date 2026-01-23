package terraform

import (
	"context"
	"fmt"

	"github.com/adrien-f/infracollect/internal/engine"
)

const (
	DataSourceStepName = "terraform_datasource"
)

type dataSourceStep struct {
	collector *Collector
	name      string
	args      map[string]any
}

func NewDataSourceStep(collector *Collector, name string, args map[string]any) (engine.Step, error) {
	return &dataSourceStep{collector: collector, name: name, args: args}, nil
}

func (s *dataSourceStep) Name() string {
	return fmt.Sprintf("%s(%s)", DataSourceStepName, s.name)
}

func (s *dataSourceStep) Kind() string {
	return DataSourceStepName
}

func (s *dataSourceStep) Resolve(ctx context.Context) (engine.Result, error) {
	data, err := s.collector.ReadDataSource(ctx, s.name, s.args)
	if err != nil {
		return engine.Result{}, err
	}

	return engine.Result{Data: data}, nil
}
