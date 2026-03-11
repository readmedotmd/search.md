package codesearch

import (
	"fmt"
	"regexp"
	"strings"
)

// langPattern defines a regex pattern for extracting a specific symbol kind.
type langPattern struct {
	re    *regexp.Regexp
	kind  SymbolKind
	name  int // capture group index for the symbol name
	scope int // capture group index for scope (-1 if none)
}

// maxSymbolsPerFile is the maximum number of symbols that can be extracted
// from a single file. Extraction stops once this limit is reached.
const maxSymbolsPerFile = 10000

// RegexExtractor extracts code symbols using language-specific regex patterns.
// It supports Go, Python, JavaScript/TypeScript, Java, Rust, C, C++, Ruby, and PHP.
// No external dependencies required.
type RegexExtractor struct {
	patterns map[string][]langPattern
}

// NewRegexExtractor creates a new RegexExtractor with built-in patterns
// for common programming languages.
func NewRegexExtractor() *RegexExtractor {
	return &RegexExtractor{
		patterns: defaultPatterns(),
	}
}

// AddLanguage registers custom patterns for a language.
// Each PatternDef maps a regex to a symbol kind.
func (e *RegexExtractor) AddLanguage(language string, defs []PatternDef) error {
	var patterns []langPattern
	for _, d := range defs {
		re, err := regexp.Compile(d.Pattern)
		if err != nil {
			return fmt.Errorf("invalid regex pattern for language %s: %w", language, err)
		}
		patterns = append(patterns, langPattern{
			re:    re,
			kind:  d.Kind,
			name:  d.NameGroup,
			scope: d.ScopeGroup,
		})
	}
	e.patterns[language] = patterns
	return nil
}

// PatternDef defines a regex pattern for symbol extraction.
type PatternDef struct {
	Pattern    string     // regex pattern
	Kind       SymbolKind // symbol kind to assign
	NameGroup  int        // capture group index for symbol name (1-based)
	ScopeGroup int        // capture group index for scope (-1 for none)
}

func (e *RegexExtractor) SupportedLanguages() []string {
	langs := make([]string, 0, len(e.patterns))
	for lang := range e.patterns {
		langs = append(langs, lang)
	}
	return langs
}

func (e *RegexExtractor) Extract(source []byte, language string) ([]Symbol, error) {
	language = strings.ToLower(language)

	// Resolve language aliases.
	language = resolveAlias(language)

	patterns, ok := e.patterns[language]
	if !ok {
		// Try all patterns if language unknown.
		return e.extractAllLanguages(source)
	}

	return e.extractWithPatterns(source, language, patterns), nil
}

func (e *RegexExtractor) extractWithPatterns(source []byte, language string, patterns []langPattern) []Symbol {
	lines := strings.Split(string(source), "\n")
	// Track best symbol per name: more specific kinds (struct, interface, etc.)
	// take priority over generic kinds (type, variable).
	bestByName := make(map[string]int) // name -> index in symbols
	var symbols []Symbol

	for lineNum, line := range lines {
		if len(symbols) >= maxSymbolsPerFile {
			break
		}
		for _, p := range patterns {
			matches := p.re.FindStringSubmatch(line)
			if matches == nil || p.name >= len(matches) {
				continue
			}
			name := matches[p.name]
			if name == "" || name == "_" {
				continue
			}

			sym := Symbol{
				Name:      name,
				Kind:      p.kind,
				Language:  language,
				Line:      lineNum + 1,
				Signature: strings.TrimSpace(line),
			}
			if p.scope >= 0 && p.scope < len(matches) && matches[p.scope] != "" {
				sym.Scope = matches[p.scope]
			}

			if idx, exists := bestByName[name]; exists {
				// Keep the more specific kind.
				if kindSpecificity(sym.Kind) > kindSpecificity(symbols[idx].Kind) {
					symbols[idx] = sym
				}
			} else {
				bestByName[name] = len(symbols)
				symbols = append(symbols, sym)
				if len(symbols) >= maxSymbolsPerFile {
					break
				}
			}
		}
	}
	return symbols
}

// kindSpecificity returns a priority score for symbol kinds.
// More specific kinds (struct, interface, class) rank higher than generic
// kinds (type, variable) so they win when multiple patterns match the same name.
func kindSpecificity(kind SymbolKind) int {
	switch kind {
	case SymbolStruct, SymbolInterface, SymbolClass, SymbolEnum, SymbolTrait:
		return 3
	case SymbolFunction, SymbolMethod, SymbolConstant:
		return 2
	case SymbolType, SymbolVariable, SymbolField, SymbolProperty:
		return 1
	default:
		return 0
	}
}

func (e *RegexExtractor) extractAllLanguages(source []byte) ([]Symbol, error) {
	// Try each language and return the one with the most symbols.
	var best []Symbol
	for lang, patterns := range e.patterns {
		syms := e.extractWithPatterns(source, lang, patterns)
		if len(syms) > len(best) {
			best = syms
		}
	}
	return best, nil
}

func resolveAlias(lang string) string {
	aliases := map[string]string{
		"golang":  "go",
		"py":      "python",
		"python3": "python",
		"js":      "javascript",
		"jsx":     "javascript",
		"ts":      "typescript",
		"tsx":     "typescript",
		"rb":      "ruby",
		"rs":      "rust",
		"cpp":     "c++",
		"cc":      "c++",
		"cxx":     "c++",
		"c#":      "csharp",
		"cs":      "csharp",
		"kt":      "kotlin",
		"kts":     "kotlin",
	}
	if resolved, ok := aliases[lang]; ok {
		return resolved
	}
	return lang
}

func defaultPatterns() map[string][]langPattern {
	return map[string][]langPattern{
		"go":         goPatterns(),
		"python":     pythonPatterns(),
		"javascript": jsPatterns(),
		"typescript": tsPatterns(),
		"java":       javaPatterns(),
		"rust":       rustPatterns(),
		"c":          cPatterns(),
		"c++":        cppPatterns(),
		"ruby":       rubyPatterns(),
		"php":        phpPatterns(),
	}
}

func goPatterns() []langPattern {
	return []langPattern{
		{regexp.MustCompile(`^func\s+\((\w+)\s+\*?(\w+)\)\s+(\w+)\s*\(`), SymbolMethod, 3, 2},
		{regexp.MustCompile(`^func\s+(\w+)\s*\(`), SymbolFunction, 1, -1},
		{regexp.MustCompile(`^type\s+(\w+)\s+struct\b`), SymbolStruct, 1, -1},
		{regexp.MustCompile(`^type\s+(\w+)\s+interface\b`), SymbolInterface, 1, -1},
		{regexp.MustCompile(`^type\s+(\w+)\s+\S`), SymbolType, 1, -1},
		{regexp.MustCompile(`^var\s+(\w+)\s+`), SymbolVariable, 1, -1},
		{regexp.MustCompile(`^\s+(\w+)\s+\w+.*` + "`" + `json:"(\w+)"`), SymbolField, 1, -1},
		{regexp.MustCompile(`^const\s+(\w+)\s*=`), SymbolConstant, 1, -1},
		{regexp.MustCompile(`^\s+(\w+)\s*=`), SymbolConstant, 1, -1},
		{regexp.MustCompile(`^package\s+(\w+)`), SymbolPackage, 1, -1},
	}
}

func pythonPatterns() []langPattern {
	return []langPattern{
		{regexp.MustCompile(`^class\s+(\w+)`), SymbolClass, 1, -1},
		{regexp.MustCompile(`^\s+def\s+(\w+)\s*\(`), SymbolMethod, 1, -1},
		{regexp.MustCompile(`^def\s+(\w+)\s*\(`), SymbolFunction, 1, -1},
		{regexp.MustCompile(`^(\w+)\s*=\s*`), SymbolVariable, 1, -1},
		{regexp.MustCompile(`^import\s+(\w+)`), SymbolImport, 1, -1},
		{regexp.MustCompile(`^from\s+(\w+)\s+import`), SymbolImport, 1, -1},
	}
}

func jsPatterns() []langPattern {
	return []langPattern{
		{regexp.MustCompile(`^(?:export\s+)?class\s+(\w+)`), SymbolClass, 1, -1},
		{regexp.MustCompile(`^(?:export\s+)?function\s+(\w+)\s*\(`), SymbolFunction, 1, -1},
		{regexp.MustCompile(`^(?:export\s+)?(?:const|let|var)\s+(\w+)\s*=\s*(?:async\s+)?function`), SymbolFunction, 1, -1},
		{regexp.MustCompile(`^(?:export\s+)?(?:const|let|var)\s+(\w+)\s*=\s*(?:async\s+)?\(`), SymbolFunction, 1, -1},
		{regexp.MustCompile(`^(?:export\s+)?(?:const|let|var)\s+(\w+)\s*=\s*\(`), SymbolFunction, 1, -1},
		{regexp.MustCompile(`^(?:export\s+)?const\s+(\w+)\s*=`), SymbolConstant, 1, -1},
		{regexp.MustCompile(`^(?:export\s+)?let\s+(\w+)\s*=`), SymbolVariable, 1, -1},
		{regexp.MustCompile(`^(?:export\s+)?var\s+(\w+)\s*=`), SymbolVariable, 1, -1},
		{regexp.MustCompile(`^\s+(\w+)\s*\(.*\)\s*\{`), SymbolMethod, 1, -1},
		{regexp.MustCompile(`^import\s+.*\s+from\s+['"]([\w./\-@]+)['"]`), SymbolImport, 1, -1},
	}
}

func tsPatterns() []langPattern {
	// TypeScript extends JavaScript patterns with type-specific patterns.
	patterns := jsPatterns()
	patterns = append(patterns,
		langPattern{regexp.MustCompile(`^(?:export\s+)?interface\s+(\w+)`), SymbolInterface, 1, -1},
		langPattern{regexp.MustCompile(`^(?:export\s+)?type\s+(\w+)\s*=`), SymbolType, 1, -1},
		langPattern{regexp.MustCompile(`^(?:export\s+)?enum\s+(\w+)`), SymbolEnum, 1, -1},
	)
	return patterns
}

func javaPatterns() []langPattern {
	return []langPattern{
		{regexp.MustCompile(`(?:public|private|protected|abstract)?\s*class\s+(\w+)`), SymbolClass, 1, -1},
		{regexp.MustCompile(`(?:public|private|protected|abstract)?\s*interface\s+(\w+)`), SymbolInterface, 1, -1},
		{regexp.MustCompile(`(?:public|private|protected|abstract)?\s*enum\s+(\w+)`), SymbolEnum, 1, -1},
		{regexp.MustCompile(`(?:public|private|protected|static|abstract|synchronized|final)\s+[\w<>\[\],\s]+\s+(\w+)\s*\(`), SymbolMethod, 1, -1},
		{regexp.MustCompile(`^\s*(\w+)\s*\([^)]*\)\s*\{`), SymbolMethod, 1, -1},
		{regexp.MustCompile(`(?:final\s+)?(?:static\s+)?[\w<>\[\]]+\s+(\w+)\s*=`), SymbolField, 1, -1},
		{regexp.MustCompile(`^package\s+([\w.]+)`), SymbolPackage, 1, -1},
		{regexp.MustCompile(`^import\s+(?:static\s+)?([\w.]+)`), SymbolImport, 1, -1},
	}
}

func rustPatterns() []langPattern {
	return []langPattern{
		{regexp.MustCompile(`^(?:pub\s+)?fn\s+(\w+)\s*[<(]`), SymbolFunction, 1, -1},
		{regexp.MustCompile(`^\s+(?:pub\s+)?fn\s+(\w+)\s*\(`), SymbolMethod, 1, -1},
		{regexp.MustCompile(`^(?:pub\s+)?struct\s+(\w+)`), SymbolStruct, 1, -1},
		{regexp.MustCompile(`^(?:pub\s+)?enum\s+(\w+)`), SymbolEnum, 1, -1},
		{regexp.MustCompile(`^(?:pub\s+)?trait\s+(\w+)`), SymbolTrait, 1, -1},
		{regexp.MustCompile(`^(?:pub\s+)?type\s+(\w+)\s*=`), SymbolType, 1, -1},
		{regexp.MustCompile(`^(?:pub\s+)?const\s+(\w+)\s*:`), SymbolConstant, 1, -1},
		{regexp.MustCompile(`^(?:pub\s+)?static\s+(\w+)\s*:`), SymbolVariable, 1, -1},
		{regexp.MustCompile(`^(?:pub\s+)?mod\s+(\w+)`), SymbolModule, 1, -1},
		// impl blocks are tracked for scope detection, not as separate symbols.
		// The struct/trait they implement is already captured by other patterns.
		{regexp.MustCompile(`^use\s+([\w:]+)`), SymbolImport, 1, -1},
	}
}

func cPatterns() []langPattern {
	return []langPattern{
		{regexp.MustCompile(`^(?:static\s+)?(?:inline\s+)?(?:extern\s+)?[\w*]+\s+(\w+)\s*\([^;]*$`), SymbolFunction, 1, -1},
		{regexp.MustCompile(`^struct\s+(\w+)\s*\{`), SymbolStruct, 1, -1},
		{regexp.MustCompile(`^typedef\s+.*\s+(\w+)\s*;`), SymbolType, 1, -1},
		{regexp.MustCompile(`^enum\s+(\w+)\s*\{`), SymbolEnum, 1, -1},
		{regexp.MustCompile(`^#define\s+(\w+)`), SymbolConstant, 1, -1},
		{regexp.MustCompile(`^(?:extern\s+)?[\w*]+\s+(\w+)\s*;`), SymbolVariable, 1, -1},
	}
}

func cppPatterns() []langPattern {
	patterns := cPatterns()
	patterns = append(patterns,
		langPattern{regexp.MustCompile(`^class\s+(\w+)`), SymbolClass, 1, -1},
		langPattern{regexp.MustCompile(`^namespace\s+(\w+)`), SymbolModule, 1, -1},
		langPattern{regexp.MustCompile(`^template\s*<.*>\s*class\s+(\w+)`), SymbolClass, 1, -1},
		langPattern{regexp.MustCompile(`^template\s*<.*>\s*struct\s+(\w+)`), SymbolStruct, 1, -1},
		langPattern{regexp.MustCompile(`(\w+)::(\w+)\s*\(`), SymbolMethod, 2, 1},
	)
	return patterns
}

func rubyPatterns() []langPattern {
	return []langPattern{
		{regexp.MustCompile(`^class\s+(\w+)`), SymbolClass, 1, -1},
		{regexp.MustCompile(`^module\s+(\w+)`), SymbolModule, 1, -1},
		{regexp.MustCompile(`^\s+def\s+self\.(\w+)`), SymbolFunction, 1, -1},
		{regexp.MustCompile(`^\s+def\s+(\w+[?!=]?)`), SymbolMethod, 1, -1},
		{regexp.MustCompile(`^def\s+(\w+[?!=]?)`), SymbolFunction, 1, -1},
		{regexp.MustCompile(`^\s+(\w+)\s*=`), SymbolVariable, 1, -1},
		{regexp.MustCompile(`^(\w+)\s*=`), SymbolConstant, 1, -1},
		{regexp.MustCompile(`^\s+attr_(?:accessor|reader|writer)\s+:(\w+)`), SymbolProperty, 1, -1},
		{regexp.MustCompile(`^require\s+['"]([\w/\-]+)['"]`), SymbolImport, 1, -1},
	}
}

func phpPatterns() []langPattern {
	return []langPattern{
		{regexp.MustCompile(`^(?:abstract\s+)?class\s+(\w+)`), SymbolClass, 1, -1},
		{regexp.MustCompile(`^interface\s+(\w+)`), SymbolInterface, 1, -1},
		{regexp.MustCompile(`^trait\s+(\w+)`), SymbolTrait, 1, -1},
		{regexp.MustCompile(`(?:public|private|protected|static)\s+function\s+(\w+)\s*\(`), SymbolMethod, 1, -1},
		{regexp.MustCompile(`^function\s+(\w+)\s*\(`), SymbolFunction, 1, -1},
		{regexp.MustCompile(`^(?:const|define)\s*\(?\s*['"]*(\w+)`), SymbolConstant, 1, -1},
		{regexp.MustCompile(`^namespace\s+([\w\\]+)`), SymbolModule, 1, -1},
		{regexp.MustCompile(`^use\s+([\w\\]+)`), SymbolImport, 1, -1},
	}
}

// CompositeExtractor tries multiple extractors and merges results.
// Useful for combining regex and tree-sitter extraction.
type CompositeExtractor struct {
	extractors []SymbolExtractor
}

// NewCompositeExtractor creates an extractor that merges results from all
// provided extractors, deduplicating by name+kind.
func NewCompositeExtractor(extractors ...SymbolExtractor) *CompositeExtractor {
	return &CompositeExtractor{extractors: extractors}
}

func (e *CompositeExtractor) SupportedLanguages() []string {
	seen := make(map[string]bool)
	var langs []string
	for _, ext := range e.extractors {
		for _, lang := range ext.SupportedLanguages() {
			if !seen[lang] {
				seen[lang] = true
				langs = append(langs, lang)
			}
		}
	}
	return langs
}

func (e *CompositeExtractor) Extract(source []byte, language string) ([]Symbol, error) {
	seen := make(map[string]bool)
	var merged []Symbol

	for _, ext := range e.extractors {
		syms, err := ext.Extract(source, language)
		if err != nil {
			continue
		}
		for _, sym := range syms {
			key := sym.Name + ":" + string(sym.Kind)
			if !seen[key] {
				seen[key] = true
				merged = append(merged, sym)
			}
		}
	}
	return merged, nil
}
