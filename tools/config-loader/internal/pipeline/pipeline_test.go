package pipeline

import (
	"context"
	"fmt"
	"testing"
)

func TestPipeline_Execute(t *testing.T) {
	p := NewPipeline(
		&mockStage{name: "stage1"},
		&mockStage{name: "stage2"},
	)

	ctx := context.Background()
	output, err := p.Execute(ctx, "input")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if output != "input" {
		t.Errorf("output = %v, want 'input'", output)
	}
}

func TestPipeline_Execute_Error(t *testing.T) {
	p := NewPipeline(
		&errorStage{name: "error"},
	)

	ctx := context.Background()
	_, err := p.Execute(ctx, "input")
	if err == nil {
		t.Error("Execute() should return error when stage fails")
	}
}

type mockStage struct {
	name string
}

func (s *mockStage) Name() string {
	return s.name
}

func (s *mockStage) Execute(ctx context.Context, input any) (any, error) {
	return input, nil
}

type errorStage struct {
	name string
}

func (s *errorStage) Name() string {
	return s.name
}

func (s *errorStage) Execute(ctx context.Context, input any) (any, error) {
	return nil, fmt.Errorf("stage error")
}
