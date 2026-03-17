package analysis

import (
	"strings"
	"unicode"
)

// LowerCaseFilter converts all token terms to lowercase.
type LowerCaseFilter struct{}

func (f *LowerCaseFilter) Filter(tokens []*Token) []*Token {
	for _, t := range tokens {
		t.Term = strings.ToLower(t.Term)
	}
	return tokens
}

// StopWordsFilter removes common stop words.
type StopWordsFilter struct {
	StopWords map[string]struct{}
}

// EnglishStopWords is the default set of English stop words.
var EnglishStopWords = map[string]struct{}{
	"a": {}, "an": {}, "and": {}, "are": {}, "as": {}, "at": {}, "be": {},
	"but": {}, "by": {}, "for": {}, "if": {}, "in": {}, "into": {}, "is": {},
	"it": {}, "no": {}, "not": {}, "of": {}, "on": {}, "or": {}, "such": {},
	"that": {}, "the": {}, "their": {}, "then": {}, "there": {}, "these": {},
	"they": {}, "this": {}, "to": {}, "was": {}, "will": {}, "with": {},
	"i": {}, "me": {}, "my": {}, "we": {}, "our": {}, "you": {}, "your": {},
	"he": {}, "him": {}, "his": {}, "she": {}, "her": {}, "its": {},
	"who": {}, "whom": {}, "what": {}, "which": {}, "do": {}, "does": {},
	"did": {}, "has": {}, "have": {}, "had": {}, "am": {}, "were": {},
	"been": {}, "being": {}, "from": {},
}

func NewStopWordsFilter() *StopWordsFilter {
	return &StopWordsFilter{StopWords: EnglishStopWords}
}

func NewStopWordsFilterWithWords(words map[string]struct{}) *StopWordsFilter {
	return &StopWordsFilter{StopWords: words}
}

func (f *StopWordsFilter) Filter(tokens []*Token) []*Token {
	rv := make([]*Token, 0, len(tokens))
	for _, t := range tokens {
		if _, isStop := f.StopWords[strings.ToLower(t.Term)]; !isStop {
			rv = append(rv, t)
		}
	}
	return rv
}

// PorterStemFilter applies a simplified Porter stemming algorithm.
type PorterStemFilter struct{}

func (f *PorterStemFilter) Filter(tokens []*Token) []*Token {
	for _, t := range tokens {
		t.Term = porterStem(t.Term)
	}
	return tokens
}

// porterStem applies a simplified Porter stemmer.
// This is not a full Porter stemmer implementation but handles the most
// common English suffixes with reasonable accuracy.
func porterStem(word string) string {
	if len(word) <= 2 {
		return word
	}

	// Step 1a: plurals
	if strings.HasSuffix(word, "sses") {
		word = word[:len(word)-2] // sses -> ss
	} else if strings.HasSuffix(word, "ies") && len(word) > 4 {
		word = word[:len(word)-2] // ies -> i (but not for 4-letter words like "ties")
	} else if !strings.HasSuffix(word, "ss") && !strings.HasSuffix(word, "us") &&
		!strings.HasSuffix(word, "is") && strings.HasSuffix(word, "s") && len(word) > 3 {
		word = word[:len(word)-1] // s ->  (but not ss, us, is)
	}

	// Step 1b: -eed, -ed, -ing
	if strings.HasSuffix(word, "eed") && len(word) > 4 {
		word = word[:len(word)-1] // eed -> ee
	} else if strings.HasSuffix(word, "ed") && len(word) > 4 && containsVowel(word[:len(word)-2]) {
		word = word[:len(word)-2]
		word = step1bCleanup(word)
	} else if strings.HasSuffix(word, "ing") && len(word) > 5 && containsVowel(word[:len(word)-3]) {
		word = word[:len(word)-3]
		word = step1bCleanup(word)
	}

	// Step 1c: y -> i when stem contains vowel
	if strings.HasSuffix(word, "y") && len(word) > 2 && containsVowel(word[:len(word)-1]) {
		word = word[:len(word)-1] + "i"
	}

	// Step 2: double suffixes
	step2 := []struct {
		suffix      string
		replacement string
		minStem     int
	}{
		{"ational", "ate", 1},
		{"tional", "tion", 1},
		{"enci", "ence", 1},
		{"anci", "ance", 1},
		{"izer", "ize", 1},
		{"isation", "ize", 1},
		{"ization", "ize", 1},
		{"ation", "ate", 1},
		{"ator", "ate", 1},
		{"alism", "al", 1},
		{"iveness", "ive", 1},
		{"fulness", "ful", 1},
		{"ousness", "ous", 1},
		{"aliti", "al", 1},
		{"iviti", "ive", 1},
		{"biliti", "ble", 1},
		{"eli", "e", 1},
		{"ousli", "ous", 1},
		{"entli", "ent", 1},
		{"alli", "al", 1},
	}

	for _, s := range step2 {
		if strings.HasSuffix(word, s.suffix) {
			stem := word[:len(word)-len(s.suffix)]
			if len(stem) > s.minStem {
				word = stem + s.replacement
			}
			break
		}
	}

	// Step 3: longer suffixes
	step3 := []struct {
		suffix      string
		replacement string
		minStem     int
	}{
		{"icate", "ic", 1},
		{"ative", "", 1},
		{"alize", "al", 1},
		{"iciti", "ic", 1},
		{"ical", "ic", 1},
		{"ful", "", 1},
		{"ness", "", 1},
	}

	for _, s := range step3 {
		if strings.HasSuffix(word, s.suffix) {
			stem := word[:len(word)-len(s.suffix)]
			if len(stem) > s.minStem {
				word = stem + s.replacement
			}
			break
		}
	}

	// Step 4: remove known suffixes if stem is long enough
	step4 := []string{
		"ement", "ment", "ent", "ant", "ence", "ance",
		"able", "ible", "ate", "ive", "ize", "ise",
		"iti", "al", "er", "ic", "ou", "ion",
	}

	for _, s := range step4 {
		if strings.HasSuffix(word, s) {
			stem := word[:len(word)-len(s)]
			if len(stem) > 2 {
				if s == "ion" {
					// Only remove -ion if preceded by s or t
					if len(stem) > 0 && (stem[len(stem)-1] == 's' || stem[len(stem)-1] == 't') {
						word = stem
					}
				} else {
					word = stem
				}
			}
			break
		}
	}

	// Step 5a: remove trailing e if stem is long enough
	if strings.HasSuffix(word, "e") && len(word) > 3 {
		word = word[:len(word)-1]
	}

	// Step 5b: reduce double l
	if strings.HasSuffix(word, "ll") && len(word) > 3 {
		word = word[:len(word)-1]
	}

	return word
}

// containsVowel returns true if the string contains at least one vowel.
func containsVowel(s string) bool {
	for _, r := range s {
		switch r {
		case 'a', 'e', 'i', 'o', 'u':
			return true
		}
	}
	return false
}

// step1bCleanup handles the special cases after removing -ed/-ing.
func step1bCleanup(word string) string {
	if strings.HasSuffix(word, "at") || strings.HasSuffix(word, "bl") || strings.HasSuffix(word, "iz") {
		return word + "e"
	}
	// Double consonant: remove one (except ll, ss, zz)
	if len(word) >= 2 {
		last := word[len(word)-1]
		prev := word[len(word)-2]
		if last == prev && last != 'l' && last != 's' && last != 'z' {
			return word[:len(word)-1]
		}
	}
	return word
}

// NGramFilter generates n-gram tokens.
type NGramFilter struct {
	MinSize int
	MaxSize int
}

func NewNGramFilter(min, max int) *NGramFilter {
	return &NGramFilter{MinSize: min, MaxSize: max}
}

func (f *NGramFilter) Filter(tokens []*Token) []*Token {
	var rv []*Token
	for _, t := range tokens {
		runes := []rune(t.Term)
		for size := f.MinSize; size <= f.MaxSize && size <= len(runes); size++ {
			for i := 0; i <= len(runes)-size; i++ {
				rv = append(rv, &Token{
					Term:     string(runes[i : i+size]),
					Position: t.Position,
					Start:    t.Start,
					End:      t.End,
					Type:     t.Type,
				})
			}
		}
	}
	return rv
}

// EdgeNGramFilter generates edge n-grams (from the start of each token).
type EdgeNGramFilter struct {
	MinSize int
	MaxSize int
}

func NewEdgeNGramFilter(min, max int) *EdgeNGramFilter {
	return &EdgeNGramFilter{MinSize: min, MaxSize: max}
}

func (f *EdgeNGramFilter) Filter(tokens []*Token) []*Token {
	var rv []*Token
	for _, t := range tokens {
		runes := []rune(t.Term)
		for size := f.MinSize; size <= f.MaxSize && size <= len(runes); size++ {
			rv = append(rv, &Token{
				Term:     string(runes[:size]),
				Position: t.Position,
				Start:    t.Start,
				End:      t.End,
				Type:     t.Type,
			})
		}
	}
	return rv
}

// TrimFilter removes leading/trailing whitespace from tokens.
type TrimFilter struct{}

func (f *TrimFilter) Filter(tokens []*Token) []*Token {
	for _, t := range tokens {
		t.Term = strings.TrimSpace(t.Term)
	}
	return tokens
}

// LengthFilter removes tokens shorter than Min or longer than Max.
type LengthFilter struct {
	Min int
	Max int
}

func (f *LengthFilter) Filter(tokens []*Token) []*Token {
	rv := make([]*Token, 0, len(tokens))
	for _, t := range tokens {
		runeLen := len([]rune(t.Term))
		if runeLen >= f.Min && (f.Max == 0 || runeLen <= f.Max) {
			rv = append(rv, t)
		}
	}
	return rv
}

// UniqueFilter removes duplicate tokens (by term).
type UniqueFilter struct{}

func (f *UniqueFilter) Filter(tokens []*Token) []*Token {
	seen := make(map[string]struct{})
	rv := make([]*Token, 0, len(tokens))
	for _, t := range tokens {
		if _, ok := seen[t.Term]; !ok {
			seen[t.Term] = struct{}{}
			rv = append(rv, t)
		}
	}
	return rv
}

// HTMLCharFilter strips HTML tags from input.
type HTMLCharFilter struct{}

func (f *HTMLCharFilter) Filter(input []byte) []byte {
	var result []byte
	inTag := false
	for _, b := range input {
		if b == '<' {
			inTag = true
			continue
		}
		if b == '>' {
			inTag = false
			result = append(result, ' ')
			continue
		}
		if !inTag {
			result = append(result, b)
		}
	}
	return result
}

// PunctuationCharFilter removes punctuation characters.
type PunctuationCharFilter struct{}

func (f *PunctuationCharFilter) Filter(input []byte) []byte {
	var result []byte
	for _, r := range string(input) {
		if !unicode.IsPunct(r) {
			result = append(result, []byte(string(r))...)
		} else {
			result = append(result, ' ')
		}
	}
	return result
}
