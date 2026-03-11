package codesearch

import (
	"bufio"
	"fmt"
	"strings"
)

// Tag represents a ctags-compatible tag entry.
type Tag struct {
	Name      string            `json:"name"`
	File      string            `json:"file,omitempty"`
	Line      int               `json:"line,omitempty"`
	Kind      string            `json:"kind"` // ctags kind letter or name
	Scope     string            `json:"scope,omitempty"`
	Signature string            `json:"signature,omitempty"`
	Extra     map[string]string `json:"extra,omitempty"` // additional ctags fields
}

// TagExtractor extracts ctags-compatible tags from source code.
// It uses the same regex patterns as RegexExtractor but produces
// tags in a format compatible with Universal Ctags.
//
// It can also parse existing ctags output files.
type TagExtractor struct {
	regex *RegexExtractor
}

// NewTagExtractor creates a new TagExtractor with built-in patterns.
func NewTagExtractor() *TagExtractor {
	return &TagExtractor{
		regex: NewRegexExtractor(),
	}
}

func (e *TagExtractor) SupportedLanguages() []string {
	return e.regex.SupportedLanguages()
}

func (e *TagExtractor) Extract(source []byte, language string) ([]Symbol, error) {
	// Extract symbols using regex, then convert to tag-compatible format.
	return e.regex.Extract(source, language)
}

// ExtractTags extracts tags in ctags-compatible format.
func (e *TagExtractor) ExtractTags(source []byte, language, filename string) ([]Tag, error) {
	symbols, err := e.regex.Extract(source, language)
	if err != nil {
		return nil, err
	}

	tags := make([]Tag, 0, len(symbols))
	for _, sym := range symbols {
		tags = append(tags, Tag{
			Name:      sym.Name,
			File:      filename,
			Line:      sym.Line,
			Kind:      symbolKindToTagKind(sym.Kind),
			Scope:     sym.Scope,
			Signature: sym.Signature,
		})
	}
	return tags, nil
}

// FormatTags formats tags in ctags output format.
func FormatTags(tags []Tag) string {
	var b strings.Builder
	for _, t := range tags {
		// Standard ctags format: name\tfile\tline;"\tkind
		b.WriteString(t.Name)
		b.WriteByte('\t')
		b.WriteString(t.File)
		b.WriteByte('\t')
		fmt.Fprintf(&b, "%d;\"", t.Line)
		b.WriteByte('\t')
		b.WriteString(t.Kind)
		if t.Scope != "" {
			b.WriteByte('\t')
			b.WriteString("scope:")
			b.WriteString(t.Scope)
		}
		if t.Signature != "" {
			b.WriteByte('\t')
			b.WriteString("signature:")
			b.WriteString(t.Signature)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// ParseTags parses a ctags-format string into Tags, then converts to Symbols.
// This allows importing existing ctags files into the search index.
func ParseTags(input string) ([]Symbol, error) {
	var symbols []Symbol
	scanner := bufio.NewScanner(strings.NewReader(input))

	for scanner.Scan() {
		line := scanner.Text()

		// Skip comments and empty lines.
		if strings.HasPrefix(line, "!_TAG_") || line == "" {
			continue
		}

		tag, err := parseTagLine(line)
		if err != nil {
			continue // skip malformed lines
		}

		symbols = append(symbols, Symbol{
			Name:      tag.Name,
			Kind:      tagKindToSymbolKind(tag.Kind),
			Scope:     tag.Scope,
			Signature: tag.Signature,
			Line:      tag.Line,
		})
	}
	return symbols, scanner.Err()
}

func parseTagLine(line string) (Tag, error) {
	// Ctags format: name\tfile\taddress;"\tkind\tfield:value...
	parts := strings.Split(line, "\t")
	if len(parts) < 3 {
		return Tag{}, fmt.Errorf("invalid ctags line: too few fields")
	}

	tag := Tag{
		Name:  parts[0],
		File:  parts[1],
		Extra: make(map[string]string),
	}

	// Parse address (line number).
	addr := parts[2]
	if idx := strings.Index(addr, ";\""); idx >= 0 {
		addr = addr[:idx]
	}
	fmt.Sscanf(addr, "%d", &tag.Line)

	// Parse kind.
	if len(parts) > 3 {
		tag.Kind = parts[3]
	}

	// Parse extended fields.
	for i := 4; i < len(parts); i++ {
		if idx := strings.Index(parts[i], ":"); idx > 0 {
			key := parts[i][:idx]
			value := parts[i][idx+1:]
			switch key {
			case "scope":
				tag.Scope = value
			case "signature":
				tag.Signature = value
			default:
				tag.Extra[key] = value
			}
		}
	}

	return tag, nil
}

// symbolKindToTagKind converts a SymbolKind to a ctags kind letter.
func symbolKindToTagKind(kind SymbolKind) string {
	switch kind {
	case SymbolFunction:
		return "f"
	case SymbolMethod:
		return "m"
	case SymbolClass:
		return "c"
	case SymbolStruct:
		return "s"
	case SymbolInterface:
		return "i"
	case SymbolType:
		return "t"
	case SymbolVariable:
		return "v"
	case SymbolConstant:
		return "d"
	case SymbolField:
		return "w"
	case SymbolImport:
		return "I"
	case SymbolPackage:
		return "p"
	case SymbolModule:
		return "n"
	case SymbolProperty:
		return "P"
	case SymbolEnum:
		return "e"
	case SymbolEnumValue:
		return "E"
	case SymbolTrait:
		return "T"
	default:
		return "v"
	}
}

// tagKindToSymbolKind converts a ctags kind letter to a SymbolKind.
func tagKindToSymbolKind(kind string) SymbolKind {
	switch kind {
	case "f", "function":
		return SymbolFunction
	case "m", "method", "member":
		return SymbolMethod
	case "c", "class":
		return SymbolClass
	case "s", "struct":
		return SymbolStruct
	case "i", "interface":
		return SymbolInterface
	case "t", "type", "typedef":
		return SymbolType
	case "v", "variable", "var":
		return SymbolVariable
	case "d", "constant", "const", "define", "macro":
		return SymbolConstant
	case "w", "field":
		return SymbolField
	case "I", "import":
		return SymbolImport
	case "p", "package":
		return SymbolPackage
	case "n", "namespace", "module":
		return SymbolModule
	case "P", "property":
		return SymbolProperty
	case "e", "enum":
		return SymbolEnum
	case "E", "enumerator":
		return SymbolEnumValue
	case "T", "trait":
		return SymbolTrait
	default:
		return SymbolVariable
	}
}

// TagFieldIndexer is a convenience wrapper that creates a SymbolFieldIndexer
// backed by a TagExtractor. It can also import ctags files.
type TagFieldIndexer struct {
	SymbolFieldIndexer
}

// NewTagFieldIndexer creates a FieldIndexer that extracts ctags-compatible tags.
func NewTagFieldIndexer() *TagFieldIndexer {
	return &TagFieldIndexer{
		SymbolFieldIndexer: SymbolFieldIndexer{
			Extractor: NewTagExtractor(),
		},
	}
}
