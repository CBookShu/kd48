package pipeline

import (
	"context"
	"fmt"
)

type Stage interface {
	Name() string
	Execute(ctx context.Context, input any) (output any, err error)
}

type Pipeline struct {
	stages []Stage
}

func NewPipeline(stages ...Stage) *Pipeline {
	return &Pipeline{stages: stages}
}

func (p *Pipeline) Execute(ctx context.Context, input any) (any, error) {
	var output any = input
	var err error

	for _, stage := range p.stages {
		output, err = stage.Execute(ctx, output)
		if err != nil {
			return nil, fmt.Errorf("stage %s failed: %w", stage.Name(), err)
		}
	}

	return output, nil
}
