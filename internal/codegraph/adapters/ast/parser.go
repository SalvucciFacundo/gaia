package ast

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"unicode"

	"gaia/internal/codegraph/domain"
)

// ParseResult encapsulates nodes and edges extracted from one or more Go files.
type ParseResult struct {
	Nodes []domain.SymbolNode
	Edges []domain.Edge
}

// Parser parses Go source files using standard go/ast and go/parser.
type Parser struct{}

// NewParser creates a new AST parser.
func NewParser() *Parser {
	return &Parser{}
}

// HashFile calculates a SHA-256 hash of a file's content.
func (p *Parser) HashFile(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

// ParseFile parses a single Go file and extracts its symbols and relationships.
func (p *Parser) ParseFile(filePath string, pkgOverride string) (*ParseResult, error) {
	fset := token.NewFileSet()
	fileNode, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse file %s: %w", filePath, err)
	}

	pkgName := fileNode.Name.Name
	if pkgOverride != "" {
		pkgName = pkgOverride
	}

	result := &ParseResult{
		Nodes: make([]domain.SymbolNode, 0),
		Edges: make([]domain.Edge, 0),
	}

	pkgID := domain.SymbolRef(pkgName)
	// Package node
	result.Nodes = append(result.Nodes, domain.SymbolNode{
		ID:         pkgID,
		Kind:       domain.KindPackage,
		Name:       pkgName,
		Package:    pkgName,
		File:       filePath,
		LineStart:  fset.Position(fileNode.Pos()).Line,
		LineEnd:    fset.Position(fileNode.End()).Line,
		Doc:        strings.TrimSpace(fileNode.Doc.Text()),
		IsExported: true,
	})

	// Track methods on structs and interface signatures for IMPLEMENTS resolution
	structMethods := make(map[string]map[string]string)   // structName -> methodName -> signature
	interfaceMethods := make(map[string]map[string]string) // interfaceName -> methodName -> signature
	declaredStructs := make(map[string]domain.SymbolRef)
	declaredInterfaces := make(map[string]domain.SymbolRef)

	// Process Imports
	for _, imp := range fileNode.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)
		edgeID := fmt.Sprintf("import-%s-%s", pkgName, importPath)
		result.Edges = append(result.Edges, domain.Edge{
			ID:       edgeID,
			SourceID: pkgID,
			TargetID: domain.SymbolRef(importPath),
			Kind:     domain.EdgeImports,
			File:     filePath,
			Line:     fset.Position(imp.Pos()).Line,
		})
	}

	// First pass: extract types, structs, interfaces, functions
	for _, decl := range fileNode.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			if d.Tok == token.TYPE {
				doc := strings.TrimSpace(d.Doc.Text())
				for _, spec := range d.Specs {
					typeSpec, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}
					typeName := typeSpec.Name.Name
					typeID := domain.SymbolRef(fmt.Sprintf("%s.%s", pkgName, typeName))
					isExported := unicode.IsUpper(rune(typeName[0]))

					switch t := typeSpec.Type.(type) {
					case *ast.StructType:
						declaredStructs[typeName] = typeID
						structNode := domain.SymbolNode{
							ID:         typeID,
							Kind:       domain.KindStruct,
							Name:       typeName,
							Package:    pkgName,
							File:       filePath,
							LineStart:  fset.Position(d.Pos()).Line,
							LineEnd:    fset.Position(d.End()).Line,
							Doc:        doc,
							IsExported: isExported,
						}
						result.Nodes = append(result.Nodes, structNode)

						// CONTAINS edge from pkg to struct
						result.Edges = append(result.Edges, domain.Edge{
							ID:       fmt.Sprintf("contains-%s-%s", pkgID, typeID),
							SourceID: pkgID,
							TargetID: typeID,
							Kind:     domain.EdgeContains,
							File:     filePath,
							Line:     structNode.LineStart,
						})

						// Extract fields
						if t.Fields != nil {
							for _, field := range t.Fields.List {
								fieldTypeStr := nodeString(fset, field.Type)
								fieldDoc := strings.TrimSpace(field.Doc.Text())
								if len(field.Names) == 0 {
									// Embedded field
									fieldName := fieldTypeStr
									fieldID := domain.SymbolRef(fmt.Sprintf("%s.%s", typeID, fieldName))
									result.Nodes = append(result.Nodes, domain.SymbolNode{
										ID:         fieldID,
										Kind:       domain.KindField,
										Name:       fieldName,
										Package:    pkgName,
										File:       filePath,
										LineStart:  fset.Position(field.Pos()).Line,
										LineEnd:    fset.Position(field.End()).Line,
										Signature:  fieldTypeStr,
										Doc:        fieldDoc,
										IsExported: unicode.IsUpper(rune(fieldName[0])),
									})
									result.Edges = append(result.Edges, domain.Edge{
										ID:       fmt.Sprintf("contains-%s-%s", typeID, fieldID),
										SourceID: typeID,
										TargetID: fieldID,
										Kind:     domain.EdgeContains,
										File:     filePath,
										Line:     fset.Position(field.Pos()).Line,
									})
								} else {
									for _, name := range field.Names {
										fieldName := name.Name
										fieldID := domain.SymbolRef(fmt.Sprintf("%s.%s", typeID, fieldName))
										result.Nodes = append(result.Nodes, domain.SymbolNode{
											ID:         fieldID,
											Kind:       domain.KindField,
											Name:       fieldName,
											Package:    pkgName,
											File:       filePath,
											LineStart:  fset.Position(field.Pos()).Line,
											LineEnd:    fset.Position(field.End()).Line,
											Signature:  fieldTypeStr,
											Doc:        fieldDoc,
											IsExported: unicode.IsUpper(rune(fieldName[0])),
										})
										result.Edges = append(result.Edges, domain.Edge{
											ID:       fmt.Sprintf("contains-%s-%s", typeID, fieldID),
											SourceID: typeID,
											TargetID: fieldID,
											Kind:     domain.EdgeContains,
											File:     filePath,
											Line:     fset.Position(field.Pos()).Line,
										})
									}
								}
							}
						}

					case *ast.InterfaceType:
						declaredInterfaces[typeName] = typeID
						if interfaceMethods[typeName] == nil {
							interfaceMethods[typeName] = make(map[string]string)
						}
						ifaceNode := domain.SymbolNode{
							ID:         typeID,
							Kind:       domain.KindInterface,
							Name:       typeName,
							Package:    pkgName,
							File:       filePath,
							LineStart:  fset.Position(d.Pos()).Line,
							LineEnd:    fset.Position(d.End()).Line,
							Doc:        doc,
							IsExported: isExported,
						}
						result.Nodes = append(result.Nodes, ifaceNode)

						result.Edges = append(result.Edges, domain.Edge{
							ID:       fmt.Sprintf("contains-%s-%s", pkgID, typeID),
							SourceID: pkgID,
							TargetID: typeID,
							Kind:     domain.EdgeContains,
							File:     filePath,
							Line:     ifaceNode.LineStart,
						})

						// Extract interface methods
						if t.Methods != nil {
							for _, method := range t.Methods.List {
								methodSig := nodeString(fset, method.Type)
								methodDoc := strings.TrimSpace(method.Doc.Text())
								for _, name := range method.Names {
									methodName := name.Name
									interfaceMethods[typeName][methodName] = methodSig
									methodID := domain.SymbolRef(fmt.Sprintf("%s.%s", typeID, methodName))
									result.Nodes = append(result.Nodes, domain.SymbolNode{
										ID:         methodID,
										Kind:       domain.KindMethod,
										Name:       methodName,
										Package:    pkgName,
										File:       filePath,
										LineStart:  fset.Position(method.Pos()).Line,
										LineEnd:    fset.Position(method.End()).Line,
										Signature:  methodSig,
										Doc:        methodDoc,
										IsExported: unicode.IsUpper(rune(methodName[0])),
									})
									result.Edges = append(result.Edges, domain.Edge{
										ID:       fmt.Sprintf("contains-%s-%s", typeID, methodID),
										SourceID: typeID,
										TargetID: methodID,
										Kind:     domain.EdgeContains,
										File:     filePath,
										Line:     fset.Position(method.Pos()).Line,
									})
								}
							}
						}

					default:
						// Type alias or basic type
						typeNode := domain.SymbolNode{
							ID:         typeID,
							Kind:       domain.KindTypeAlias,
							Name:       typeName,
							Package:    pkgName,
							File:       filePath,
							LineStart:  fset.Position(d.Pos()).Line,
							LineEnd:    fset.Position(d.End()).Line,
							Signature:  nodeString(fset, typeSpec.Type),
							Doc:        doc,
							IsExported: isExported,
						}
						result.Nodes = append(result.Nodes, typeNode)
						result.Edges = append(result.Edges, domain.Edge{
							ID:       fmt.Sprintf("contains-%s-%s", pkgID, typeID),
							SourceID: pkgID,
							TargetID: typeID,
							Kind:     domain.EdgeContains,
							File:     filePath,
							Line:     typeNode.LineStart,
						})
					}
				}
			}

		case *ast.FuncDecl:
			funcName := d.Name.Name
			isExported := unicode.IsUpper(rune(funcName[0]))
			sig := funcSignature(fset, d.Type)
			doc := strings.TrimSpace(d.Doc.Text())
			startLine := fset.Position(d.Pos()).Line
			endLine := fset.Position(d.End()).Line

			var sourceFuncID domain.SymbolRef

			if d.Recv == nil {
				// Standalone function
				funcID := domain.SymbolRef(fmt.Sprintf("%s.%s", pkgName, funcName))
				sourceFuncID = funcID
				result.Nodes = append(result.Nodes, domain.SymbolNode{
					ID:         funcID,
					Kind:       domain.KindFunc,
					Name:       funcName,
					Package:    pkgName,
					File:       filePath,
					LineStart:  startLine,
					LineEnd:    endLine,
					Signature:  sig,
					Doc:        doc,
					IsExported: isExported,
				})

				result.Edges = append(result.Edges, domain.Edge{
					ID:       fmt.Sprintf("contains-%s-%s", pkgID, funcID),
					SourceID: pkgID,
					TargetID: funcID,
					Kind:     domain.EdgeContains,
					File:     filePath,
					Line:     startLine,
				})
			} else {
				// Method with receiver
				recvType := receiverTypeName(d.Recv)
				recvTypeID := domain.SymbolRef(fmt.Sprintf("%s.%s", pkgName, recvType))
				methodID := domain.SymbolRef(fmt.Sprintf("%s.%s.%s", pkgName, recvType, funcName))
				sourceFuncID = methodID

				if structMethods[recvType] == nil {
					structMethods[recvType] = make(map[string]string)
				}
				structMethods[recvType][funcName] = sig

				result.Nodes = append(result.Nodes, domain.SymbolNode{
					ID:         methodID,
					Kind:       domain.KindMethod,
					Name:       funcName,
					Package:    pkgName,
					File:       filePath,
					LineStart:  startLine,
					LineEnd:    endLine,
					Signature:  sig,
					Doc:        doc,
					IsExported: isExported,
				})

				result.Edges = append(result.Edges, domain.Edge{
					ID:       fmt.Sprintf("contains-%s-%s", recvTypeID, methodID),
					SourceID: recvTypeID,
					TargetID: methodID,
					Kind:     domain.EdgeContains,
					File:     filePath,
					Line:     startLine,
				})

				result.Edges = append(result.Edges, domain.Edge{
					ID:       fmt.Sprintf("recv-%s-%s", recvTypeID, methodID),
					SourceID: recvTypeID,
					TargetID: methodID,
					Kind:     domain.EdgeReceiverOf,
					File:     filePath,
					Line:     startLine,
				})
			}

			// Extract Calls within function body
			if d.Body != nil {
				ast.Inspect(d.Body, func(n ast.Node) bool {
					callExpr, ok := n.(*ast.CallExpr)
					if !ok {
						return true
					}
					callLine := fset.Position(callExpr.Pos()).Line

					switch fun := callExpr.Fun.(type) {
					case *ast.Ident:
						// Call to local function in package
						targetID := domain.SymbolRef(fmt.Sprintf("%s.%s", pkgName, fun.Name))
						edgeID := fmt.Sprintf("call-%s-%s-%d", sourceFuncID, targetID, callLine)
						result.Edges = append(result.Edges, domain.Edge{
							ID:       edgeID,
							SourceID: sourceFuncID,
							TargetID: targetID,
							Kind:     domain.EdgeCalls,
							File:     filePath,
							Line:     callLine,
						})

					case *ast.SelectorExpr:
						// Call on object, e.g. s.Greet() or pkg.Func()
						selName := fun.Sel.Name
						if xIdent, ok := fun.X.(*ast.Ident); ok {
							// If xIdent matches receiver in method or a variable
							var targetID domain.SymbolRef
							if d.Recv != nil && receiverVarName(d.Recv) == xIdent.Name {
								// Calling method on self
								recvType := receiverTypeName(d.Recv)
								targetID = domain.SymbolRef(fmt.Sprintf("%s.%s.%s", pkgName, recvType, selName))
							} else {
								// Could be pkg.Func or var.Method -> try pkgName.Type.Method or pkgName.Method
								targetID = domain.SymbolRef(fmt.Sprintf("%s.%s", xIdent.Name, selName))
							}
							edgeID := fmt.Sprintf("call-%s-%s-%d", sourceFuncID, targetID, callLine)
							result.Edges = append(result.Edges, domain.Edge{
								ID:       edgeID,
								SourceID: sourceFuncID,
								TargetID: targetID,
								Kind:     domain.EdgeCalls,
								File:     filePath,
								Line:     callLine,
							})
						}
					}
					return true
				})
			}
		}
	}

	// Resolve IMPLEMENTS edges
	for sName, sID := range declaredStructs {
		sMethodMap := structMethods[sName]
		if sMethodMap == nil {
			sMethodMap = make(map[string]string)
		}
		for iName, iID := range declaredInterfaces {
			iMethodMap := interfaceMethods[iName]
			if len(iMethodMap) == 0 {
				continue
			}
			implements := true
			for mName := range iMethodMap {
				if _, ok := sMethodMap[mName]; !ok {
					implements = false
					break
				}
			}
			if implements {
				result.Edges = append(result.Edges, domain.Edge{
					ID:       fmt.Sprintf("impl-%s-%s", sID, iID),
					SourceID: sID,
					TargetID: iID,
					Kind:     domain.EdgeImplements,
					File:     filePath,
					Line:     1,
				})
			}
		}
	}

	return result, nil
}

func receiverTypeName(recv *ast.FieldList) string {
	if recv == nil || len(recv.List) == 0 {
		return ""
	}
	t := recv.List[0].Type
	if star, ok := t.(*ast.StarExpr); ok {
		t = star.X
	}
	if ident, ok := t.(*ast.Ident); ok {
		return ident.Name
	}
	return ""
}

func receiverVarName(recv *ast.FieldList) string {
	if recv == nil || len(recv.List) == 0 || len(recv.List[0].Names) == 0 {
		return ""
	}
	return recv.List[0].Names[0].Name
}

func funcSignature(fset *token.FileSet, funcType *ast.FuncType) string {
	var buf bytes.Buffer
	buf.WriteString("(")
	if funcType.Params != nil {
		for i, field := range funcType.Params.List {
			if i > 0 {
				buf.WriteString(", ")
			}
			fieldType := nodeString(fset, field.Type)
			if len(field.Names) > 0 {
				names := make([]string, len(field.Names))
				for j, n := range field.Names {
					names[j] = n.Name
				}
				buf.WriteString(strings.Join(names, ", "))
				buf.WriteString(" ")
			}
			buf.WriteString(fieldType)
		}
	}
	buf.WriteString(")")
	if funcType.Results != nil && len(funcType.Results.List) > 0 {
		buf.WriteString(" ")
		if len(funcType.Results.List) == 1 && len(funcType.Results.List[0].Names) == 0 {
			buf.WriteString(nodeString(fset, funcType.Results.List[0].Type))
		} else {
			buf.WriteString("(")
			for i, field := range funcType.Results.List {
				if i > 0 {
					buf.WriteString(", ")
				}
				fieldType := nodeString(fset, field.Type)
				if len(field.Names) > 0 {
					names := make([]string, len(field.Names))
					for j, n := range field.Names {
						names[j] = n.Name
					}
					buf.WriteString(strings.Join(names, ", "))
					buf.WriteString(" ")
				}
				buf.WriteString(fieldType)
			}
			buf.WriteString(")")
		}
	}
	return buf.String()
}

func nodeString(fset *token.FileSet, node ast.Node) string {
	if node == nil {
		return ""
	}
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, node); err != nil {
		return ""
	}
	return buf.String()
}
