package codesearch

import "fmt"

// Tree-sitter integration via interface.
//
// This plugin does NOT directly depend on tree-sitter. Instead, it defines
// interfaces that users implement using their preferred tree-sitter Go binding
// (e.g., github.com/smacker/go-tree-sitter or github.com/tree-sitter/go-tree-sitter).
//
// Example with go-tree-sitter:
//
//	import (
//	    sitter "github.com/smacker/go-tree-sitter"
//	    "github.com/smacker/go-tree-sitter/golang"
//	    "github.com/readmedotmd/search.md/plugin/codesearch"
//	)
//
//	// Implement the codesearch.ASTParser interface:
//	type goParser struct {
//	    parser *sitter.Parser
//	}
//
//	func (p *goParser) Parse(source []byte) (*codesearch.ASTNode, error) {
//	    p.parser.SetLanguage(golang.GetLanguage())
//	    tree, _ := p.parser.ParseCtx(context.Background(), nil, source)
//	    return convertNode(tree.RootNode(), source), nil
//	}
//
//	// Then use it:
//	parser := &goParser{parser: sitter.NewParser()}
//	extractor := codesearch.NewTreeSitterExtractor(parser, "go")
//	idx, _ := searchmd.New(store, mapping,
//	    codesearch.WithSymbolIndexer(extractor),
//	)

// ASTNode represents a node in a parsed syntax tree.
// This is a simplified, tree-sitter-compatible representation that any
// parser can produce.
type ASTNode struct {
	// Type is the grammar node type (e.g., "function_declaration", "identifier").
	Type string

	// Content is the text content of this node.
	Content string

	// Position information.
	StartByte uint32
	EndByte   uint32
	StartRow  uint32
	StartCol  uint32
	EndRow    uint32
	EndCol    uint32

	// Children are the child nodes.
	Children []*ASTNode

	// FieldName is the grammar field name (e.g., "name", "body", "parameters").
	FieldName string
}

// ChildByFieldName returns the first child with the given field name.
func (n *ASTNode) ChildByFieldName(name string) *ASTNode {
	for _, child := range n.Children {
		if child.FieldName == name {
			return child
		}
	}
	return nil
}

// ChildrenByType returns all children with the given node type.
func (n *ASTNode) ChildrenByType(nodeType string) []*ASTNode {
	var result []*ASTNode
	for _, child := range n.Children {
		if child.Type == nodeType {
			result = append(result, child)
		}
	}
	return result
}

// Walk calls fn for every node in the tree (depth-first).
// If fn returns false, walking stops.
func (n *ASTNode) Walk(fn func(*ASTNode) bool) {
	if !fn(n) {
		return
	}
	for _, child := range n.Children {
		child.Walk(fn)
	}
}

// ASTParser parses source code into an AST.
// Implement this interface using your preferred tree-sitter binding.
type ASTParser interface {
	// Parse parses source code and returns the root AST node.
	Parse(source []byte) (*ASTNode, error)
}

// ExtractionRule defines how to extract a symbol from an AST node.
type ExtractionRule struct {
	// NodeType is the tree-sitter node type to match (e.g., "function_declaration").
	NodeType string

	// Kind is the symbol kind to assign.
	Kind SymbolKind

	// NameField is the child field name containing the symbol name (e.g., "name").
	NameField string

	// ScopeParentTypes are node types to look up in ancestors for scope.
	// E.g., ["class_definition", "impl_item"] to detect class/impl scope.
	ScopeParentTypes []string

	// ScopeNameField is the field name for the scope's name in the parent node.
	ScopeNameField string
}

// TreeSitterExtractor extracts symbols from source code using a tree-sitter parser.
type TreeSitterExtractor struct {
	parsers map[string]ASTParser // language -> parser
	rules   map[string][]ExtractionRule
}

// NewTreeSitterExtractor creates a new TreeSitterExtractor.
// Register parsers per language with AddLanguage().
func NewTreeSitterExtractor() *TreeSitterExtractor {
	return &TreeSitterExtractor{
		parsers: make(map[string]ASTParser),
		rules:   defaultExtractionRules(),
	}
}

// AddLanguage registers a parser and optional custom rules for a language.
// If rules is nil, built-in rules are used.
func (e *TreeSitterExtractor) AddLanguage(language string, parser ASTParser, rules []ExtractionRule) {
	e.parsers[language] = parser
	if rules != nil {
		e.rules[language] = rules
	}
}

func (e *TreeSitterExtractor) SupportedLanguages() []string {
	langs := make([]string, 0, len(e.parsers))
	for lang := range e.parsers {
		langs = append(langs, lang)
	}
	return langs
}

func (e *TreeSitterExtractor) Extract(source []byte, language string) ([]Symbol, error) {
	language = resolveAlias(language)

	parser, ok := e.parsers[language]
	if !ok {
		return nil, fmt.Errorf("no tree-sitter parser registered for language %q", language)
	}

	root, err := parser.Parse(source)
	if err != nil {
		return nil, fmt.Errorf("tree-sitter parse error: %w", err)
	}

	rules, ok := e.rules[language]
	if !ok {
		// Fall back to generic rules.
		rules = genericRules()
	}

	var symbols []Symbol
	ruleMap := make(map[string]ExtractionRule)
	for _, r := range rules {
		ruleMap[r.NodeType] = r
	}

	// Collect all scope-providing node types across all rules so we know
	// which ancestor types to track on the scope stack.
	scopeNameFields := make(map[string]string) // nodeType -> nameField
	for _, r := range rules {
		for _, pt := range r.ScopeParentTypes {
			if _, exists := scopeNameFields[pt]; !exists {
				scopeNameFields[pt] = r.ScopeNameField
			}
		}
	}

	// Use an iterative DFS with a scope stack instead of Walk + findScope
	// so that scope resolution is O(1) per symbol instead of O(tree_nodes).
	type frame struct {
		node       *ASTNode
		childIndex int
	}
	type scopeEntry struct {
		nodeType string
		name     string
	}

	stack := []frame{{node: root, childIndex: 0}}
	var scopeStack []scopeEntry

	for len(stack) > 0 {
		top := &stack[len(stack)-1]

		if top.childIndex == 0 {
			// First visit to this node: process it.
			node := top.node

			// Push scope if this node is a scope-providing type.
			if nameField, isScopeType := scopeNameFields[node.Type]; isScopeType {
				scopeName := ""
				if nameNode := node.ChildByFieldName(nameField); nameNode != nil {
					scopeName = nameNode.Content
				}
				scopeStack = append(scopeStack, scopeEntry{nodeType: node.Type, name: scopeName})
			}

			// Extract symbol if this node matches a rule.
			if rule, ok := ruleMap[node.Type]; ok {
				nameNode := node.ChildByFieldName(rule.NameField)
				if nameNode != nil {
					sym := Symbol{
						Name:      nameNode.Content,
						Kind:      rule.Kind,
						Language:  language,
						Line:      int(node.StartRow) + 1,
						StartByte: int(node.StartByte),
						EndByte:   int(node.EndByte),
					}

					// Resolve scope from the stack in O(1) amortized.
					if len(rule.ScopeParentTypes) > 0 {
						parentTypeSet := make(map[string]bool, len(rule.ScopeParentTypes))
						for _, pt := range rule.ScopeParentTypes {
							parentTypeSet[pt] = true
						}
						// Walk the scope stack top-down to find the nearest matching scope.
						for i := len(scopeStack) - 1; i >= 0; i-- {
							if parentTypeSet[scopeStack[i].nodeType] {
								sym.Scope = scopeStack[i].name
								break
							}
						}
					}

					symbols = append(symbols, sym)
					if len(symbols) >= maxSymbolsPerFile {
						return symbols, nil
					}
				}
			}
		}

		// Advance to the next child, or pop if done.
		if top.childIndex < len(top.node.Children) {
			child := top.node.Children[top.childIndex]
			top.childIndex++
			stack = append(stack, frame{node: child, childIndex: 0})
		} else {
			// Leaving this node: pop scope if we pushed one.
			if _, isScopeType := scopeNameFields[top.node.Type]; isScopeType {
				scopeStack = scopeStack[:len(scopeStack)-1]
			}
			stack = stack[:len(stack)-1]
		}
	}

	return symbols, nil
}

// defaultExtractionRules returns built-in extraction rules for common languages.
func defaultExtractionRules() map[string][]ExtractionRule {
	return map[string][]ExtractionRule{
		"go":         goExtractionRules(),
		"python":     pythonExtractionRules(),
		"javascript": jsExtractionRules(),
		"typescript": tsExtractionRules(),
		"java":       javaExtractionRules(),
		"rust":       rustExtractionRules(),
		"c":          cExtractionRules(),
		"c++":        cppExtractionRules(),
		"ruby":       rubyExtractionRules(),
	}
}

func genericRules() []ExtractionRule {
	return []ExtractionRule{
		{NodeType: "function_declaration", Kind: SymbolFunction, NameField: "name"},
		{NodeType: "function_definition", Kind: SymbolFunction, NameField: "name"},
		{NodeType: "method_declaration", Kind: SymbolMethod, NameField: "name"},
		{NodeType: "method_definition", Kind: SymbolMethod, NameField: "name"},
		{NodeType: "class_declaration", Kind: SymbolClass, NameField: "name"},
		{NodeType: "class_definition", Kind: SymbolClass, NameField: "name"},
		{NodeType: "interface_declaration", Kind: SymbolInterface, NameField: "name"},
		{NodeType: "type_declaration", Kind: SymbolType, NameField: "name"},
		{NodeType: "struct_item", Kind: SymbolStruct, NameField: "name"},
		{NodeType: "enum_item", Kind: SymbolEnum, NameField: "name"},
	}
}

func goExtractionRules() []ExtractionRule {
	return []ExtractionRule{
		{NodeType: "function_declaration", Kind: SymbolFunction, NameField: "name"},
		{NodeType: "method_declaration", Kind: SymbolMethod, NameField: "name"},
		{NodeType: "type_declaration", Kind: SymbolType, NameField: "name"},
		{NodeType: "type_spec", Kind: SymbolType, NameField: "name"},
		{NodeType: "const_spec", Kind: SymbolConstant, NameField: "name"},
		{NodeType: "var_spec", Kind: SymbolVariable, NameField: "name"},
		{NodeType: "package_clause", Kind: SymbolPackage, NameField: "name"},
		{NodeType: "import_spec", Kind: SymbolImport, NameField: "path"},
		{NodeType: "field_declaration", Kind: SymbolField, NameField: "name"},
	}
}

func pythonExtractionRules() []ExtractionRule {
	return []ExtractionRule{
		{NodeType: "function_definition", Kind: SymbolFunction, NameField: "name",
			ScopeParentTypes: []string{"class_definition"}, ScopeNameField: "name"},
		{NodeType: "class_definition", Kind: SymbolClass, NameField: "name"},
		{NodeType: "assignment", Kind: SymbolVariable, NameField: "left"},
		{NodeType: "import_statement", Kind: SymbolImport, NameField: "name"},
		{NodeType: "import_from_statement", Kind: SymbolImport, NameField: "module_name"},
	}
}

func jsExtractionRules() []ExtractionRule {
	return []ExtractionRule{
		{NodeType: "function_declaration", Kind: SymbolFunction, NameField: "name"},
		{NodeType: "class_declaration", Kind: SymbolClass, NameField: "name"},
		{NodeType: "method_definition", Kind: SymbolMethod, NameField: "name",
			ScopeParentTypes: []string{"class_declaration", "class"}, ScopeNameField: "name"},
		{NodeType: "variable_declarator", Kind: SymbolVariable, NameField: "name"},
		{NodeType: "lexical_declaration", Kind: SymbolVariable, NameField: "name"},
		{NodeType: "export_statement", Kind: SymbolVariable, NameField: "declaration"},
	}
}

func tsExtractionRules() []ExtractionRule {
	rules := jsExtractionRules()
	rules = append(rules,
		ExtractionRule{NodeType: "interface_declaration", Kind: SymbolInterface, NameField: "name"},
		ExtractionRule{NodeType: "type_alias_declaration", Kind: SymbolType, NameField: "name"},
		ExtractionRule{NodeType: "enum_declaration", Kind: SymbolEnum, NameField: "name"},
	)
	return rules
}

func javaExtractionRules() []ExtractionRule {
	return []ExtractionRule{
		{NodeType: "class_declaration", Kind: SymbolClass, NameField: "name"},
		{NodeType: "interface_declaration", Kind: SymbolInterface, NameField: "name"},
		{NodeType: "enum_declaration", Kind: SymbolEnum, NameField: "name"},
		{NodeType: "method_declaration", Kind: SymbolMethod, NameField: "name",
			ScopeParentTypes: []string{"class_declaration", "interface_declaration"}, ScopeNameField: "name"},
		{NodeType: "constructor_declaration", Kind: SymbolMethod, NameField: "name"},
		{NodeType: "field_declaration", Kind: SymbolField, NameField: "declarator"},
		{NodeType: "package_declaration", Kind: SymbolPackage, NameField: "name"},
		{NodeType: "import_declaration", Kind: SymbolImport, NameField: "name"},
	}
}

func rustExtractionRules() []ExtractionRule {
	return []ExtractionRule{
		{NodeType: "function_item", Kind: SymbolFunction, NameField: "name",
			ScopeParentTypes: []string{"impl_item"}, ScopeNameField: "type"},
		{NodeType: "struct_item", Kind: SymbolStruct, NameField: "name"},
		{NodeType: "enum_item", Kind: SymbolEnum, NameField: "name"},
		{NodeType: "trait_item", Kind: SymbolTrait, NameField: "name"},
		{NodeType: "type_item", Kind: SymbolType, NameField: "name"},
		{NodeType: "const_item", Kind: SymbolConstant, NameField: "name"},
		{NodeType: "static_item", Kind: SymbolVariable, NameField: "name"},
		{NodeType: "mod_item", Kind: SymbolModule, NameField: "name"},
		{NodeType: "impl_item", Kind: SymbolType, NameField: "type"},
		{NodeType: "use_declaration", Kind: SymbolImport, NameField: "argument"},
	}
}

func cExtractionRules() []ExtractionRule {
	return []ExtractionRule{
		{NodeType: "function_definition", Kind: SymbolFunction, NameField: "declarator"},
		{NodeType: "declaration", Kind: SymbolVariable, NameField: "declarator"},
		{NodeType: "struct_specifier", Kind: SymbolStruct, NameField: "name"},
		{NodeType: "enum_specifier", Kind: SymbolEnum, NameField: "name"},
		{NodeType: "type_definition", Kind: SymbolType, NameField: "declarator"},
		{NodeType: "preproc_def", Kind: SymbolConstant, NameField: "name"},
	}
}

func cppExtractionRules() []ExtractionRule {
	rules := cExtractionRules()
	rules = append(rules,
		ExtractionRule{NodeType: "class_specifier", Kind: SymbolClass, NameField: "name"},
		ExtractionRule{NodeType: "namespace_definition", Kind: SymbolModule, NameField: "name"},
		ExtractionRule{NodeType: "template_declaration", Kind: SymbolType, NameField: "name"},
	)
	return rules
}

func rubyExtractionRules() []ExtractionRule {
	return []ExtractionRule{
		{NodeType: "method", Kind: SymbolMethod, NameField: "name",
			ScopeParentTypes: []string{"class", "module"}, ScopeNameField: "name"},
		{NodeType: "singleton_method", Kind: SymbolFunction, NameField: "name"},
		{NodeType: "class", Kind: SymbolClass, NameField: "name"},
		{NodeType: "module", Kind: SymbolModule, NameField: "name"},
		{NodeType: "assignment", Kind: SymbolVariable, NameField: "left"},
	}
}
