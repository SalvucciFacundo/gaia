package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"gaia/internal/codegraph/domain"
	"gaia/internal/codegraph/ports"
)

var _ ports.QueryEnginePort = (*QueryEngine)(nil)

// QueryEngine implements ports.QueryEnginePort over SQLite.
type QueryEngine struct {
	store *Store
}

// NewQueryEngine creates a QueryEngine backed by a SQLite store.
func NewQueryEngine(store *Store) *QueryEngine {
	return &QueryEngine{store: store}
}

// FindCallers finds all symbols calling the given target symbol.
func (q *QueryEngine) FindCallers(ctx context.Context, target domain.SymbolRef) ([]domain.CallerResult, error) {
	if _, err := q.store.GetNode(ctx, target); err != nil {
		return nil, err
	}

	query := `
		SELECT n.id, n.kind, n.name, n.package, n.file, n.line_start, n.line_end, n.signature, n.doc, n.is_exported, e.file, e.line
		FROM edges e
		JOIN nodes n ON e.source_id = n.id
		WHERE e.target_id = ? AND e.kind = 'CALLS'
	`
	rows, err := q.store.DB().QueryContext(ctx, query, string(target))
	if err != nil {
		return nil, fmt.Errorf("failed to query callers: %w", err)
	}
	defer rows.Close()

	var results []domain.CallerResult
	for rows.Next() {
		var n domain.SymbolNode
		var idStr string
		var sig, doc sql.NullString
		var edgeFile sql.NullString
		var edgeLine sql.NullInt64

		if err := rows.Scan(&idStr, &n.Kind, &n.Name, &n.Package, &n.File, &n.LineStart, &n.LineEnd, &sig, &doc, &n.IsExported, &edgeFile, &edgeLine); err != nil {
			return nil, fmt.Errorf("failed to scan caller row: %w", err)
		}
		n.ID = domain.SymbolRef(idStr)
		if sig.Valid {
			n.Signature = sig.String
		}
		if doc.Valid {
			n.Doc = doc.String
		}

		res := domain.CallerResult{
			Caller:   n,
			CallFile: edgeFile.String,
			CallLine: int(edgeLine.Int64),
		}
		results = append(results, res)
	}

	return results, rows.Err()
}

// FindCallees finds all symbols called by the given source symbol.
func (q *QueryEngine) FindCallees(ctx context.Context, source domain.SymbolRef) ([]domain.CalleeResult, error) {
	if _, err := q.store.GetNode(ctx, source); err != nil {
		return nil, err
	}

	query := `
		SELECT n.id, n.kind, n.name, n.package, n.file, n.line_start, n.line_end, n.signature, n.doc, n.is_exported, e.file, e.line
		FROM edges e
		JOIN nodes n ON e.target_id = n.id
		WHERE e.source_id = ? AND e.kind = 'CALLS'
	`
	rows, err := q.store.DB().QueryContext(ctx, query, string(source))
	if err != nil {
		return nil, fmt.Errorf("failed to query callees: %w", err)
	}
	defer rows.Close()

	var results []domain.CalleeResult
	for rows.Next() {
		var n domain.SymbolNode
		var idStr string
		var sig, doc sql.NullString
		var edgeFile sql.NullString
		var edgeLine sql.NullInt64

		if err := rows.Scan(&idStr, &n.Kind, &n.Name, &n.Package, &n.File, &n.LineStart, &n.LineEnd, &sig, &doc, &n.IsExported, &edgeFile, &edgeLine); err != nil {
			return nil, fmt.Errorf("failed to scan callee row: %w", err)
		}
		n.ID = domain.SymbolRef(idStr)
		if sig.Valid {
			n.Signature = sig.String
		}
		if doc.Valid {
			n.Doc = doc.String
		}

		res := domain.CalleeResult{
			Callee:   n,
			CallFile: edgeFile.String,
			CallLine: int(edgeLine.Int64),
		}
		results = append(results, res)
	}

	return results, rows.Err()
}

// FindImplementations returns implementations of an interface, or interfaces implemented by a struct.
func (q *QueryEngine) FindImplementations(ctx context.Context, symbolRef domain.SymbolRef) ([]domain.SymbolNode, error) {
	node, err := q.store.GetNode(ctx, symbolRef)
	if err != nil {
		return nil, err
	}

	var query string
	if node.Kind == domain.KindInterface {
		query = `
			SELECT n.id, n.kind, n.name, n.package, n.file, n.line_start, n.line_end, n.signature, n.doc, n.is_exported
			FROM edges e
			JOIN nodes n ON e.source_id = n.id
			WHERE e.target_id = ? AND e.kind = 'IMPLEMENTS'
		`
	} else {
		query = `
			SELECT n.id, n.kind, n.name, n.package, n.file, n.line_start, n.line_end, n.signature, n.doc, n.is_exported
			FROM edges e
			JOIN nodes n ON e.target_id = n.id
			WHERE e.source_id = ? AND e.kind = 'IMPLEMENTS'
		`
	}

	rows, err := q.store.DB().QueryContext(ctx, query, string(symbolRef))
	if err != nil {
		return nil, fmt.Errorf("failed to query implementations: %w", err)
	}
	defer rows.Close()

	var results []domain.SymbolNode
	for rows.Next() {
		var n domain.SymbolNode
		var idStr string
		var sig, doc sql.NullString
		if err := rows.Scan(&idStr, &n.Kind, &n.Name, &n.Package, &n.File, &n.LineStart, &n.LineEnd, &sig, &doc, &n.IsExported); err != nil {
			return nil, fmt.Errorf("failed to scan implementation row: %w", err)
		}
		n.ID = domain.SymbolRef(idStr)
		if sig.Valid {
			n.Signature = sig.String
		}
		if doc.Valid {
			n.Doc = doc.String
		}
		results = append(results, n)
	}

	return results, rows.Err()
}

// GetCallHierarchy traverses call graphs recursively with cycle prevention.
func (q *QueryEngine) GetCallHierarchy(ctx context.Context, root domain.SymbolRef, direction domain.HierarchyDirection, maxDepth int) (*domain.CallHierarchyTree, error) {
	rootNode, err := q.store.GetNode(ctx, root)
	if err != nil {
		return nil, err
	}

	if maxDepth <= 0 {
		maxDepth = 3
	}

	tree := &domain.CallHierarchyTree{
		Root:      *rootNode,
		Direction: direction,
		Nodes:     make([]*domain.CallHierarchyNode, 0),
	}

	visited := make(map[domain.SymbolRef]bool)
	visited[root] = true

	nodes, err := q.buildHierarchy(ctx, root, direction, 1, maxDepth, visited)
	if err != nil {
		return nil, err
	}
	tree.Nodes = nodes
	return tree, nil
}

func (q *QueryEngine) buildHierarchy(ctx context.Context, current domain.SymbolRef, direction domain.HierarchyDirection, depth, maxDepth int, visited map[domain.SymbolRef]bool) ([]*domain.CallHierarchyNode, error) {
	if depth > maxDepth {
		return nil, nil
	}

	var nodes []*domain.CallHierarchyNode

	if direction == domain.DirectionUpstream {
		callers, err := q.FindCallers(ctx, current)
		if err != nil && err != domain.ErrSymbolNotFound {
			return nil, err
		}
		for _, c := range callers {
			child := &domain.CallHierarchyNode{
				Symbol:   c.Caller,
				CallSite: c.CallFile,
				CallLine: c.CallLine,
			}
			if visited[c.Caller.ID] {
				child.IsCircular = true
			} else {
				newVisited := copyVisited(visited)
				newVisited[c.Caller.ID] = true
				subChildren, err := q.buildHierarchy(ctx, c.Caller.ID, direction, depth+1, maxDepth, newVisited)
				if err != nil {
					return nil, err
				}
				child.Children = subChildren
			}
			nodes = append(nodes, child)
		}
	} else {
		callees, err := q.FindCallees(ctx, current)
		if err != nil && err != domain.ErrSymbolNotFound {
			return nil, err
		}
		for _, c := range callees {
			child := &domain.CallHierarchyNode{
				Symbol:   c.Callee,
				CallSite: c.CallFile,
				CallLine: c.CallLine,
			}
			if visited[c.Callee.ID] {
				child.IsCircular = true
			} else {
				newVisited := copyVisited(visited)
				newVisited[c.Callee.ID] = true
				subChildren, err := q.buildHierarchy(ctx, c.Callee.ID, direction, depth+1, maxDepth, newVisited)
				if err != nil {
					return nil, err
				}
				child.Children = subChildren
			}
			nodes = append(nodes, child)
		}
	}

	return nodes, nil
}

func copyVisited(src map[domain.SymbolRef]bool) map[domain.SymbolRef]bool {
	dst := make(map[domain.SymbolRef]bool, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// LookupSymbol queries symbols using criteria filters.
func (q *QueryEngine) LookupSymbol(ctx context.Context, criteria domain.SymbolFilter) ([]domain.SymbolNode, error) {
	var whereClauses []string
	var args []interface{}

	if criteria.Name != "" {
		whereClauses = append(whereClauses, "name = ?")
		args = append(args, criteria.Name)
	}
	if criteria.Prefix != "" {
		whereClauses = append(whereClauses, "name LIKE ?")
		args = append(args, criteria.Prefix+"%")
	}
	if criteria.Kind != "" {
		whereClauses = append(whereClauses, "kind = ?")
		args = append(args, criteria.Kind)
	}
	if criteria.Package != "" {
		whereClauses = append(whereClauses, "package = ?")
		args = append(args, criteria.Package)
	}
	if criteria.File != "" {
		whereClauses = append(whereClauses, "file = ?")
		args = append(args, criteria.File)
	}
	if criteria.ExportedOnly {
		whereClauses = append(whereClauses, "is_exported = 1")
	}

	query := "SELECT id, kind, name, package, file, line_start, line_end, signature, doc, is_exported FROM nodes"
	if len(whereClauses) > 0 {
		query += " WHERE " + strings.Join(whereClauses, " AND ")
	}
	query += " ORDER BY is_exported DESC, name ASC LIMIT 100"

	rows, err := q.store.DB().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup symbols: %w", err)
	}
	defer rows.Close()

	var results []domain.SymbolNode
	for rows.Next() {
		var n domain.SymbolNode
		var idStr string
		var sig, doc sql.NullString
		if err := rows.Scan(&idStr, &n.Kind, &n.Name, &n.Package, &n.File, &n.LineStart, &n.LineEnd, &sig, &doc, &n.IsExported); err != nil {
			return nil, fmt.Errorf("failed to scan symbol row: %w", err)
		}
		n.ID = domain.SymbolRef(idStr)
		if sig.Valid {
			n.Signature = sig.String
		}
		if doc.Valid {
			n.Doc = doc.String
		}
		results = append(results, n)
	}

	return results, rows.Err()
}

// GetStructDetails returns struct fields, methods, and interface implementations.
func (q *QueryEngine) GetStructDetails(ctx context.Context, structRef domain.SymbolRef) (*domain.StructDetails, error) {
	node, err := q.store.GetNode(ctx, structRef)
	if err != nil {
		return nil, err
	}

	details := &domain.StructDetails{
		Node:       *node,
		Fields:     make([]domain.FieldInfo, 0),
		Methods:    make([]domain.MethodInfo, 0),
		Implements: make([]domain.SymbolNode, 0),
	}

	// 1. Get Fields & Methods via CONTAINS edges
	queryMembers := `
		SELECT n.id, n.kind, n.name, n.signature, n.doc, n.is_exported
		FROM edges e
		JOIN nodes n ON e.target_id = n.id
		WHERE e.source_id = ? AND e.kind = 'CONTAINS'
	`
	rows, err := q.store.DB().QueryContext(ctx, queryMembers, string(structRef))
	if err != nil {
		return nil, fmt.Errorf("failed to query struct members: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var idStr, kind, name string
		var sig, doc sql.NullString
		var isExported bool
		if err := rows.Scan(&idStr, &kind, &name, &sig, &doc, &isExported); err != nil {
			return nil, fmt.Errorf("failed to scan member: %w", err)
		}

		sigStr := ""
		if sig.Valid {
			sigStr = sig.String
		}
		docStr := ""
		if doc.Valid {
			docStr = doc.String
		}

		if kind == domain.KindField {
			details.Fields = append(details.Fields, domain.FieldInfo{
				Name:       name,
				Type:       sigStr,
				Doc:        docStr,
				IsExported: isExported,
			})
		} else if kind == domain.KindMethod {
			details.Methods = append(details.Methods, domain.MethodInfo{
				Name:       name,
				Signature:  sigStr,
				Doc:        docStr,
				IsExported: isExported,
			})
		}
	}

	// 2. Get Implements interfaces
	ifaces, err := q.FindImplementations(ctx, structRef)
	if err == nil {
		details.Implements = ifaces
	}

	return details, nil
}
