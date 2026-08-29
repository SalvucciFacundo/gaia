package domain_test

import (
	"testing"

	"gaia/internal/codegraph/domain"
)

func TestSymbolNode_Validation(t *testing.T) {
	tests := []struct {
		name    string
		node    domain.SymbolNode
		wantErr bool
	}{
		{
			name: "valid node",
			node: domain.SymbolNode{
				ID:         "gaia/pkg.MyFunc",
				Kind:       domain.KindFunc,
				Name:       "MyFunc",
				Package:    "gaia/pkg",
				File:       "pkg/file.go",
				LineStart:  10,
				LineEnd:    20,
				Signature:  "() error",
				Doc:        "MyFunc does something",
				IsExported: true,
			},
			wantErr: false,
		},
		{
			name: "missing ID",
			node: domain.SymbolNode{
				Kind:    domain.KindFunc,
				Name:    "MyFunc",
				Package: "gaia/pkg",
				File:    "pkg/file.go",
			},
			wantErr: true,
		},
		{
			name: "invalid line range",
			node: domain.SymbolNode{
				ID:        "gaia/pkg.MyFunc",
				Kind:      domain.KindFunc,
				Name:      "MyFunc",
				Package:   "gaia/pkg",
				File:      "pkg/file.go",
				LineStart: 25,
				LineEnd:   10,
			},
			wantErr: true,
		},
		{
			name: "missing kind",
			node: domain.SymbolNode{
				ID:      "gaia/pkg.MyFunc",
				Name:    "MyFunc",
				Package: "gaia/pkg",
				File:    "pkg/file.go",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.node.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("SymbolNode.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestEdge_Validation(t *testing.T) {
	tests := []struct {
		name    string
		edge    domain.Edge
		wantErr bool
	}{
		{
			name: "valid edge",
			edge: domain.Edge{
				ID:       "edge-1",
				SourceID: "gaia/pkg.Caller",
				TargetID: "gaia/pkg.Callee",
				Kind:     domain.EdgeCalls,
				File:     "pkg/caller.go",
				Line:     42,
			},
			wantErr: false,
		},
		{
			name: "missing source",
			edge: domain.Edge{
				ID:       "edge-2",
				TargetID: "gaia/pkg.Callee",
				Kind:     domain.EdgeCalls,
			},
			wantErr: true,
		},
		{
			name: "missing target",
			edge: domain.Edge{
				ID:       "edge-3",
				SourceID: "gaia/pkg.Caller",
				Kind:     domain.EdgeCalls,
			},
			wantErr: true,
		},
		{
			name: "invalid kind",
			edge: domain.Edge{
				ID:       "edge-4",
				SourceID: "gaia/pkg.Caller",
				TargetID: "gaia/pkg.Callee",
				Kind:     "UNKNOWN_KIND",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.edge.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Edge.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSymbolFilter_Matches(t *testing.T) {
	node := domain.SymbolNode{
		ID:         "gaia/pkg.OrderService",
		Kind:       domain.KindStruct,
		Name:       "OrderService",
		Package:    "gaia/pkg",
		File:       "pkg/service.go",
		IsExported: true,
	}

	filterMatching := domain.SymbolFilter{
		Name: "OrderService",
		Kind: domain.KindStruct,
	}
	if !filterMatching.Matches(node) {
		t.Errorf("expected filter to match node")
	}

	filterPrefix := domain.SymbolFilter{
		Prefix: "Order",
	}
	if !filterPrefix.Matches(node) {
		t.Errorf("expected prefix filter to match node")
	}

	filterExported := domain.SymbolFilter{
		ExportedOnly: true,
	}
	if !filterExported.Matches(node) {
		t.Errorf("expected exported filter to match node")
	}

	filterNonMatching := domain.SymbolFilter{
		Name: "UserService",
	}
	if filterNonMatching.Matches(node) {
		t.Errorf("expected filter not to match node")
	}
}
