package analysis

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// MaxTokenizerInputSize is the maximum input size (in bytes) that tokenizers
// will process. Inputs exceeding this limit are truncated before tokenization.
const MaxTokenizerInputSize = 10 * 1024 * 1024 // 10MB

// truncateInput truncates input to MaxTokenizerInputSize if it exceeds the limit.
func truncateInput(input []byte) []byte {
	if len(input) > MaxTokenizerInputSize {
		return input[:MaxTokenizerInputSize]
	}
	return input
}

// UnicodeTokenizer splits text on unicode word boundaries.
type UnicodeTokenizer struct{}

func (t *UnicodeTokenizer) Tokenize(input []byte) []*Token {
	input = truncateInput(input)
	var tokens []*Token
	pos := 1
	text := string(input)
	start := 0
	inToken := false

	for i, r := range text {
		isWordChar := unicode.IsLetter(r) || unicode.IsDigit(r)
		if isWordChar {
			if !inToken {
				start = i
				inToken = true
			}
		} else {
			if inToken {
				tokens = append(tokens, &Token{
					Term:     text[start:i],
					Position: pos,
					Start:    start,
					End:      i,
					Type:     AlphaNumeric,
				})
				pos++
				inToken = false
			}
		}
	}
	// Handle last token
	if inToken {
		tokens = append(tokens, &Token{
			Term:     text[start:],
			Position: pos,
			Start:    start,
			End:      len(text),
			Type:     AlphaNumeric,
		})
	}

	return tokens
}

// WhitespaceTokenizer splits text on whitespace.
type WhitespaceTokenizer struct{}

func (t *WhitespaceTokenizer) Tokenize(input []byte) []*Token {
	input = truncateInput(input)
	var tokens []*Token
	pos := 1
	text := string(input)
	start := 0
	inToken := false

	for i, r := range text {
		if !unicode.IsSpace(r) {
			if !inToken {
				start = i
				inToken = true
			}
		} else {
			if inToken {
				tokens = append(tokens, &Token{
					Term:     text[start:i],
					Position: pos,
					Start:    start,
					End:      i,
					Type:     AlphaNumeric,
				})
				pos++
				inToken = false
			}
		}
	}
	if inToken {
		tokens = append(tokens, &Token{
			Term:     text[start:],
			Position: pos,
			Start:    start,
			End:      len(text),
			Type:     AlphaNumeric,
		})
	}

	return tokens
}

// KeywordTokenizer returns the entire input as a single token.
type KeywordTokenizer struct{}

func (t *KeywordTokenizer) Tokenize(input []byte) []*Token {
	input = truncateInput(input)
	if len(input) == 0 {
		return nil
	}
	return []*Token{
		{
			Term:     string(input),
			Position: 1,
			Start:    0,
			End:      len(input),
			Type:     AlphaNumeric,
		},
	}
}

// CodeTokenizer splits code into searchable tokens, handling:
// - camelCase and PascalCase splitting
// - snake_case splitting
// - dot notation (e.g., pkg.Func)
// - Preserves original token plus sub-tokens
type CodeTokenizer struct{}

func (t *CodeTokenizer) Tokenize(input []byte) []*Token {
	input = truncateInput(input)
	var tokens []*Token
	pos := 1
	text := string(input)

	// First pass: split on whitespace and punctuation to get raw tokens
	var rawTokens []struct {
		text  string
		start int
		end   int
	}

	start := 0
	inToken := false
	for i, r := range text {
		isTokenChar := unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '.'
		if isTokenChar {
			if !inToken {
				start = i
				inToken = true
			}
		} else {
			if inToken {
				rawTokens = append(rawTokens, struct {
					text  string
					start int
					end   int
				}{text[start:i], start, i})
				inToken = false
			}
			// Also index operators and punctuation as tokens
			if !unicode.IsSpace(r) {
				size := utf8.RuneLen(r)
				rawTokens = append(rawTokens, struct {
					text  string
					start int
					end   int
				}{text[i : i+size], i, i + size})
			}
		}
	}
	if inToken {
		rawTokens = append(rawTokens, struct {
			text  string
			start int
			end   int
		}{text[start:], start, len(text)})
	}

	// Second pass: split each raw token further (camelCase, snake_case, dots)
	for _, rt := range rawTokens {
		// Add the full token
		tokens = append(tokens, &Token{
			Term:     strings.ToLower(rt.text),
			Position: pos,
			Start:    rt.start,
			End:      rt.end,
			Type:     AlphaNumeric,
		})

		// Split on dots
		if strings.Contains(rt.text, ".") {
			parts := strings.Split(rt.text, ".")
			for _, part := range parts {
				if part != "" {
					tokens = append(tokens, &Token{
						Term:     strings.ToLower(part),
						Position: pos,
						Start:    rt.start,
						End:      rt.end,
						Type:     AlphaNumeric,
					})
				}
			}
		}

		// Split on underscores
		if strings.Contains(rt.text, "_") {
			parts := strings.Split(rt.text, "_")
			for _, part := range parts {
				if part != "" {
					tokens = append(tokens, &Token{
						Term:     strings.ToLower(part),
						Position: pos,
						Start:    rt.start,
						End:      rt.end,
						Type:     AlphaNumeric,
					})
				}
			}
		}

		// Split camelCase / PascalCase
		subTokens := splitCamelCase(rt.text)
		if len(subTokens) > 1 {
			for _, st := range subTokens {
				tokens = append(tokens, &Token{
					Term:     strings.ToLower(st),
					Position: pos,
					Start:    rt.start,
					End:      rt.end,
					Type:     AlphaNumeric,
				})
			}
		}

		pos++
	}

	return tokens
}

// splitCamelCase splits "camelCaseString" into ["camel", "Case", "String"].
func splitCamelCase(s string) []string {
	var parts []string
	runes := []rune(s)
	start := 0
	for i := 1; i < len(runes); i++ {
		if unicode.IsUpper(runes[i]) && (i+1 >= len(runes) || unicode.IsLower(runes[i+1]) || unicode.IsLower(runes[i-1])) {
			part := string(runes[start:i])
			if part != "" && part != "_" {
				parts = append(parts, part)
			}
			start = i
		}
	}
	if start < len(runes) {
		part := string(runes[start:])
		if part != "" && part != "_" {
			parts = append(parts, part)
		}
	}
	return parts
}
