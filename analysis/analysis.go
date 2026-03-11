package analysis

// Token represents a single token produced by analysis.
type Token struct {
	Term     string
	Position int // 1-based position in the token stream
	Start    int // byte offset of token start in original text
	End      int // byte offset of token end in original text
	Type     TokenType
}

// TokenType classifies tokens.
type TokenType int

const (
	AlphaNumeric TokenType = iota
	Numeric
	Boolean
	DateTime
	Shingle
	Single
	Ideographic
)

// CharFilter transforms input text before tokenization.
type CharFilter interface {
	Filter(input []byte) []byte
}

// Tokenizer splits input text into tokens.
type Tokenizer interface {
	Tokenize(input []byte) []*Token
}

// TokenFilter transforms a stream of tokens.
type TokenFilter interface {
	Filter(tokens []*Token) []*Token
}

// Analyzer is a complete text analysis pipeline.
type Analyzer struct {
	CharFilters  []CharFilter
	Tokenizer    Tokenizer
	TokenFilters []TokenFilter
}

// Analyze runs the full analysis pipeline on input text.
func (a *Analyzer) Analyze(input []byte) []*Token {
	// Apply char filters
	for _, cf := range a.CharFilters {
		input = cf.Filter(input)
	}

	// Tokenize
	tokens := a.Tokenizer.Tokenize(input)

	// Apply token filters
	for _, tf := range a.TokenFilters {
		tokens = tf.Filter(tokens)
	}

	return tokens
}

// TokenFrequency tracks term frequency and positions in a field.
type TokenFrequency struct {
	Term      string
	Frequency int
	Positions []*TokenPosition
}

// TokenPosition records the location of a token occurrence.
type TokenPosition struct {
	Position int // 1-based position
	Start    int // byte offset start
	End      int // byte offset end
}

// TokenFrequencies computes term frequencies from a token stream.
func TokenFrequencies(tokens []*Token, includeTermVectors bool) map[string]*TokenFrequency {
	rv := make(map[string]*TokenFrequency)
	for _, token := range tokens {
		tf, exists := rv[token.Term]
		if !exists {
			tf = &TokenFrequency{
				Term: token.Term,
			}
			rv[token.Term] = tf
		}
		tf.Frequency++
		if includeTermVectors {
			tf.Positions = append(tf.Positions, &TokenPosition{
				Position: token.Position,
				Start:    token.Start,
				End:      token.End,
			})
		}
	}
	return rv
}
