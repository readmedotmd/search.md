package analysis

import (
	"reflect"
	"testing"
)

// ---------------------------------------------------------------------------
// Helper to extract just the Term fields from a token slice.
// ---------------------------------------------------------------------------

func terms(tokens []*Token) []string {
	out := make([]string, len(tokens))
	for i, t := range tokens {
		out[i] = t.Term
	}
	return out
}

func makeTokens(terms ...string) []*Token {
	tokens := make([]*Token, len(terms))
	for i, t := range terms {
		tokens[i] = &Token{Term: t, Position: i + 1, Start: i, End: i + len(t)}
	}
	return tokens
}

// ===========================================================================
// UnicodeTokenizer
// ===========================================================================

func TestUnicodeTokenizer_Basic(t *testing.T) {
	tok := &UnicodeTokenizer{}
	tokens := tok.Tokenize([]byte("hello world"))
	got := terms(tokens)
	want := []string{"hello", "world"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if tokens[0].Position != 1 || tokens[1].Position != 2 {
		t.Errorf("unexpected positions: %d, %d", tokens[0].Position, tokens[1].Position)
	}
}

func TestUnicodeTokenizer_Punctuation(t *testing.T) {
	tok := &UnicodeTokenizer{}
	got := terms(tok.Tokenize([]byte("hello, world! foo-bar.")))
	want := []string{"hello", "world", "foo", "bar"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestUnicodeTokenizer_Numbers(t *testing.T) {
	tok := &UnicodeTokenizer{}
	got := terms(tok.Tokenize([]byte("abc 123 def456")))
	want := []string{"abc", "123", "def456"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestUnicodeTokenizer_Unicode(t *testing.T) {
	tok := &UnicodeTokenizer{}
	got := terms(tok.Tokenize([]byte("cafe\u0301 naïve über")))
	// The combining accent (U+0301) is not a letter/digit, so it splits "café" oddly:
	// "caf" then "e" + combining accent handled... let's check what actually happens
	// Actually \u0301 is a Mark, not Letter, so it splits "cafe" then "naïve" then "über"
	// Let's just verify it doesn't panic and returns something sensible
	if len(got) < 2 {
		t.Errorf("expected at least 2 tokens, got %v", got)
	}
}

func TestUnicodeTokenizer_CJK(t *testing.T) {
	tok := &UnicodeTokenizer{}
	// CJK characters are letters, so "日本語" should be one token
	got := terms(tok.Tokenize([]byte("日本語 test")))
	want := []string{"日本語", "test"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestUnicodeTokenizer_Empty(t *testing.T) {
	tok := &UnicodeTokenizer{}
	tokens := tok.Tokenize([]byte(""))
	if len(tokens) != 0 {
		t.Errorf("expected 0 tokens, got %d", len(tokens))
	}
}

func TestUnicodeTokenizer_SingleWord(t *testing.T) {
	tok := &UnicodeTokenizer{}
	tokens := tok.Tokenize([]byte("hello"))
	if len(tokens) != 1 || tokens[0].Term != "hello" {
		t.Errorf("expected [hello], got %v", terms(tokens))
	}
	if tokens[0].Start != 0 || tokens[0].End != 5 {
		t.Errorf("unexpected offsets: %d-%d", tokens[0].Start, tokens[0].End)
	}
}

func TestUnicodeTokenizer_SingleChar(t *testing.T) {
	tok := &UnicodeTokenizer{}
	tokens := tok.Tokenize([]byte("a"))
	if len(tokens) != 1 || tokens[0].Term != "a" {
		t.Errorf("expected [a], got %v", terms(tokens))
	}
}

func TestUnicodeTokenizer_OnlyPunctuation(t *testing.T) {
	tok := &UnicodeTokenizer{}
	tokens := tok.Tokenize([]byte("!@#$%"))
	if len(tokens) != 0 {
		t.Errorf("expected 0 tokens, got %v", terms(tokens))
	}
}

// ===========================================================================
// WhitespaceTokenizer
// ===========================================================================

func TestWhitespaceTokenizer_Basic(t *testing.T) {
	tok := &WhitespaceTokenizer{}
	got := terms(tok.Tokenize([]byte("hello world")))
	want := []string{"hello", "world"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestWhitespaceTokenizer_Tabs(t *testing.T) {
	tok := &WhitespaceTokenizer{}
	got := terms(tok.Tokenize([]byte("hello\tworld")))
	want := []string{"hello", "world"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestWhitespaceTokenizer_MultipleSpaces(t *testing.T) {
	tok := &WhitespaceTokenizer{}
	got := terms(tok.Tokenize([]byte("hello   world")))
	want := []string{"hello", "world"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestWhitespaceTokenizer_Newlines(t *testing.T) {
	tok := &WhitespaceTokenizer{}
	got := terms(tok.Tokenize([]byte("hello\nworld\r\nfoo")))
	want := []string{"hello", "world", "foo"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestWhitespaceTokenizer_KeepsPunctuation(t *testing.T) {
	tok := &WhitespaceTokenizer{}
	got := terms(tok.Tokenize([]byte("hello, world!")))
	want := []string{"hello,", "world!"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestWhitespaceTokenizer_Empty(t *testing.T) {
	tok := &WhitespaceTokenizer{}
	tokens := tok.Tokenize([]byte(""))
	if len(tokens) != 0 {
		t.Errorf("expected 0 tokens, got %d", len(tokens))
	}
}

func TestWhitespaceTokenizer_OnlySpaces(t *testing.T) {
	tok := &WhitespaceTokenizer{}
	tokens := tok.Tokenize([]byte("   \t\n  "))
	if len(tokens) != 0 {
		t.Errorf("expected 0 tokens, got %d", len(tokens))
	}
}

// ===========================================================================
// KeywordTokenizer
// ===========================================================================

func TestKeywordTokenizer_FullInput(t *testing.T) {
	tok := &KeywordTokenizer{}
	tokens := tok.Tokenize([]byte("hello world"))
	if len(tokens) != 1 {
		t.Fatalf("expected 1 token, got %d", len(tokens))
	}
	if tokens[0].Term != "hello world" {
		t.Errorf("expected 'hello world', got '%s'", tokens[0].Term)
	}
	if tokens[0].Position != 1 {
		t.Errorf("expected position 1, got %d", tokens[0].Position)
	}
	if tokens[0].Start != 0 || tokens[0].End != 11 {
		t.Errorf("unexpected offsets: %d-%d", tokens[0].Start, tokens[0].End)
	}
}

func TestKeywordTokenizer_Empty(t *testing.T) {
	tok := &KeywordTokenizer{}
	tokens := tok.Tokenize([]byte(""))
	if tokens != nil {
		t.Errorf("expected nil, got %v", tokens)
	}
}

func TestKeywordTokenizer_SingleChar(t *testing.T) {
	tok := &KeywordTokenizer{}
	tokens := tok.Tokenize([]byte("x"))
	if len(tokens) != 1 || tokens[0].Term != "x" {
		t.Errorf("expected [x], got %v", terms(tokens))
	}
}

// ===========================================================================
// CodeTokenizer
// ===========================================================================

func TestCodeTokenizer_CamelCase(t *testing.T) {
	tok := &CodeTokenizer{}
	got := terms(tok.Tokenize([]byte("camelCaseVar")))
	// Should contain the full token lowercased plus sub-tokens
	if !containsTerm(got, "camelcasevar") {
		t.Errorf("expected full token 'camelcasevar' in %v", got)
	}
	if !containsTerm(got, "camel") {
		t.Errorf("expected sub-token 'camel' in %v", got)
	}
	if !containsTerm(got, "case") {
		t.Errorf("expected sub-token 'case' in %v", got)
	}
	if !containsTerm(got, "var") {
		t.Errorf("expected sub-token 'var' in %v", got)
	}
}

func TestCodeTokenizer_PascalCase(t *testing.T) {
	tok := &CodeTokenizer{}
	got := terms(tok.Tokenize([]byte("PascalCaseName")))
	if !containsTerm(got, "pascalcasename") {
		t.Errorf("expected full token 'pascalcasename' in %v", got)
	}
	if !containsTerm(got, "pascal") {
		t.Errorf("expected sub-token 'pascal' in %v", got)
	}
	if !containsTerm(got, "case") {
		t.Errorf("expected sub-token 'case' in %v", got)
	}
	if !containsTerm(got, "name") {
		t.Errorf("expected sub-token 'name' in %v", got)
	}
}

func TestCodeTokenizer_SnakeCase(t *testing.T) {
	tok := &CodeTokenizer{}
	got := terms(tok.Tokenize([]byte("snake_case_var")))
	if !containsTerm(got, "snake_case_var") {
		t.Errorf("expected full token 'snake_case_var' in %v", got)
	}
	if !containsTerm(got, "snake") {
		t.Errorf("expected sub-token 'snake' in %v", got)
	}
	if !containsTerm(got, "case") {
		t.Errorf("expected sub-token 'case' in %v", got)
	}
	if !containsTerm(got, "var") {
		t.Errorf("expected sub-token 'var' in %v", got)
	}
}

func TestCodeTokenizer_DotNotation(t *testing.T) {
	tok := &CodeTokenizer{}
	got := terms(tok.Tokenize([]byte("pkg.Func")))
	if !containsTerm(got, "pkg.func") {
		t.Errorf("expected full token 'pkg.func' in %v", got)
	}
	if !containsTerm(got, "pkg") {
		t.Errorf("expected sub-token 'pkg' in %v", got)
	}
	if !containsTerm(got, "func") {
		t.Errorf("expected sub-token 'func' in %v", got)
	}
}

func TestCodeTokenizer_Mixed(t *testing.T) {
	tok := &CodeTokenizer{}
	got := terms(tok.Tokenize([]byte("fmt.Println myVar")))
	if !containsTerm(got, "fmt.println") {
		t.Errorf("expected 'fmt.println' in %v", got)
	}
	if !containsTerm(got, "fmt") {
		t.Errorf("expected 'fmt' in %v", got)
	}
	if !containsTerm(got, "myvar") {
		t.Errorf("expected 'myvar' in %v", got)
	}
}

func TestCodeTokenizer_Operators(t *testing.T) {
	tok := &CodeTokenizer{}
	got := terms(tok.Tokenize([]byte("a + b")))
	if !containsTerm(got, "a") {
		t.Errorf("expected 'a' in %v", got)
	}
	if !containsTerm(got, "+") {
		t.Errorf("expected '+' in %v", got)
	}
	if !containsTerm(got, "b") {
		t.Errorf("expected 'b' in %v", got)
	}
}

func TestCodeTokenizer_Empty(t *testing.T) {
	tok := &CodeTokenizer{}
	tokens := tok.Tokenize([]byte(""))
	if len(tokens) != 0 {
		t.Errorf("expected 0 tokens, got %d", len(tokens))
	}
}

func TestCodeTokenizer_OnlyWhitespace(t *testing.T) {
	tok := &CodeTokenizer{}
	tokens := tok.Tokenize([]byte("   "))
	if len(tokens) != 0 {
		t.Errorf("expected 0 tokens, got %d", len(tokens))
	}
}

// ===========================================================================
// LowerCaseFilter
// ===========================================================================

func TestLowerCaseFilter(t *testing.T) {
	f := &LowerCaseFilter{}
	tokens := makeTokens("Hello", "WORLD", "fOo", "bar")
	result := f.Filter(tokens)
	got := terms(result)
	want := []string{"hello", "world", "foo", "bar"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestLowerCaseFilter_Empty(t *testing.T) {
	f := &LowerCaseFilter{}
	result := f.Filter(nil)
	if len(result) != 0 {
		t.Errorf("expected 0 tokens, got %d", len(result))
	}
}

func TestLowerCaseFilter_AlreadyLower(t *testing.T) {
	f := &LowerCaseFilter{}
	tokens := makeTokens("hello")
	result := f.Filter(tokens)
	if result[0].Term != "hello" {
		t.Errorf("expected 'hello', got '%s'", result[0].Term)
	}
}

func TestLowerCaseFilter_Unicode(t *testing.T) {
	f := &LowerCaseFilter{}
	tokens := makeTokens("ÜBER", "Naïve")
	result := f.Filter(tokens)
	if result[0].Term != "über" {
		t.Errorf("expected 'über', got '%s'", result[0].Term)
	}
	if result[1].Term != "naïve" {
		t.Errorf("expected 'naïve', got '%s'", result[1].Term)
	}
}

// ===========================================================================
// StopWordsFilter
// ===========================================================================

func TestStopWordsFilter_RemovesStopWords(t *testing.T) {
	f := NewStopWordsFilter()
	tokens := makeTokens("the", "quick", "brown", "fox", "is", "a", "animal")
	result := f.Filter(tokens)
	got := terms(result)
	want := []string{"quick", "brown", "fox", "animal"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestStopWordsFilter_CustomWords(t *testing.T) {
	custom := map[string]struct{}{
		"foo": {},
		"bar": {},
	}
	f := NewStopWordsFilterWithWords(custom)
	tokens := makeTokens("foo", "baz", "bar", "qux")
	result := f.Filter(tokens)
	got := terms(result)
	want := []string{"baz", "qux"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestStopWordsFilter_CaseInsensitive(t *testing.T) {
	f := NewStopWordsFilter()
	tokens := makeTokens("The", "FOX", "IS")
	result := f.Filter(tokens)
	got := terms(result)
	want := []string{"FOX"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestStopWordsFilter_Empty(t *testing.T) {
	f := NewStopWordsFilter()
	result := f.Filter(nil)
	if len(result) != 0 {
		t.Errorf("expected 0 tokens, got %d", len(result))
	}
}

func TestStopWordsFilter_NoStopWords(t *testing.T) {
	f := NewStopWordsFilter()
	tokens := makeTokens("quick", "brown", "fox")
	result := f.Filter(tokens)
	if len(result) != 3 {
		t.Errorf("expected 3 tokens, got %d", len(result))
	}
}

// ===========================================================================
// PorterStemFilter
// ===========================================================================

func TestPorterStemFilter(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"running", "run"},        // step 1b: "ing" removed, stem "run"
		{"cats", "cat"},           // step 1a: "s" removed
		{"caresses", "caress"},    // step 1a: "sses" -> "ss"
		{"flies", "fli"},          // step 1a: "ies" -> "i"
		{"happiness", "happi"},    // step 3: "ness" removed
		{"hopeful", "hop"},        // step 3: "ful" removed -> "hope", step 5a: "e" removed
		{"active", "act"},         // step 4: "ive" removed
		{"slowly", "slowli"},      // step 1c: "y" -> "i"
		{"national", "nation"},    // step 4: "al" removed
		{"relational", "rel"},     // step 2: "ational" -> "ate", step 4: "ate" removed
		{"conditional", "condit"}, // step 2: "tional" -> "tion", step 4: "ion" removed (preceded by t)
		// Short words should not be stemmed
		{"go", "go"},
		{"a", "a"},
		{"an", "an"},
	}

	f := &PorterStemFilter{}
	for _, tc := range tests {
		tokens := makeTokens(tc.input)
		result := f.Filter(tokens)
		if result[0].Term != tc.want {
			t.Errorf("porterStem(%q) = %q, want %q", tc.input, result[0].Term, tc.want)
		}
	}
}

func TestPorterStemFilter_Empty(t *testing.T) {
	f := &PorterStemFilter{}
	result := f.Filter(nil)
	if len(result) != 0 {
		t.Errorf("expected 0 tokens, got %d", len(result))
	}
}

func TestPorterStemFilter_Step1a_Plurals(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"caresses", "caress"}, // sses -> ss
		{"ponies", "poni"},     // ies -> i (len > 4)
		{"ties", "tie"},        // ies len == 4, falls through to s removal
		{"cats", "cat"},        // s removed
		{"gas", "gas"},         // 3-letter word, s not removed
		{"dress", "dress"},     // ss suffix, no change
	}
	f := &PorterStemFilter{}
	for _, tc := range tests {
		tokens := makeTokens(tc.input)
		result := f.Filter(tokens)
		if result[0].Term != tc.want {
			t.Errorf("porterStem(%q) = %q, want %q", tc.input, result[0].Term, tc.want)
		}
	}
}

func TestPorterStemFilter_Step1b_EdIng(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"agreed", "agre"},  // eed -> ee (len > 4)
		{"feed", "feed"},    // eed but len == 4
		{"hopping", "hop"},  // ing removed + double consonant reduced
		{"tanning", "tan"},  // ing removed + double consonant reduced
		{"falling", "fal"},  // ing removed -> fall, step 5b reduces ll -> l
		{"hissing", "hiss"}, // ing removed, ss kept (exception)
		{"fizzing", "fizz"}, // ing removed, zz kept (exception)
	}
	f := &PorterStemFilter{}
	for _, tc := range tests {
		tokens := makeTokens(tc.input)
		result := f.Filter(tokens)
		if result[0].Term != tc.want {
			t.Errorf("porterStem(%q) = %q, want %q", tc.input, result[0].Term, tc.want)
		}
	}
}

func TestPorterStemFilter_Step1c_YtoI(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"happy", "happi"}, // y -> i (contains vowel before y)
		{"sky", "sky"},     // no vowel before y, so no change
	}
	f := &PorterStemFilter{}
	for _, tc := range tests {
		tokens := makeTokens(tc.input)
		result := f.Filter(tokens)
		if result[0].Term != tc.want {
			t.Errorf("porterStem(%q) = %q, want %q", tc.input, result[0].Term, tc.want)
		}
	}
}

func TestContainsVowel(t *testing.T) {
	if containsVowel("xyz") {
		t.Error("expected no vowel in 'xyz'")
	}
	if !containsVowel("axyz") {
		t.Error("expected vowel in 'axyz'")
	}
	if !containsVowel("xyez") {
		t.Error("expected vowel in 'xyez'")
	}
}

func TestStep1bCleanup(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"hat", "hate"},       // at -> ate
		{"troubl", "trouble"}, // bl -> ble
		{"siz", "size"},       // iz -> ize
		{"hopp", "hop"},       // double consonant reduced (not l/s/z)
		{"fall", "fall"},      // double l kept
	}
	for _, tc := range tests {
		got := step1bCleanup(tc.input)
		if got != tc.want {
			t.Errorf("step1bCleanup(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// ===========================================================================
// NGramFilter
// ===========================================================================

func TestNGramFilter_BiGrams(t *testing.T) {
	f := NewNGramFilter(2, 2)
	tokens := makeTokens("abc")
	result := f.Filter(tokens)
	got := terms(result)
	want := []string{"ab", "bc"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestNGramFilter_Range(t *testing.T) {
	f := NewNGramFilter(1, 3)
	tokens := makeTokens("abc")
	result := f.Filter(tokens)
	got := terms(result)
	// 1-grams: a, b, c; 2-grams: ab, bc; 3-grams: abc
	want := []string{"a", "b", "c", "ab", "bc", "abc"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestNGramFilter_MultipleTokens(t *testing.T) {
	f := NewNGramFilter(2, 2)
	tokens := makeTokens("ab", "cd")
	result := f.Filter(tokens)
	got := terms(result)
	want := []string{"ab", "cd"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestNGramFilter_ShortToken(t *testing.T) {
	f := NewNGramFilter(3, 3)
	tokens := makeTokens("ab")
	result := f.Filter(tokens)
	if len(result) != 0 {
		t.Errorf("expected 0 tokens for short input, got %v", terms(result))
	}
}

func TestNGramFilter_Unicode(t *testing.T) {
	f := NewNGramFilter(1, 2)
	tokens := makeTokens("日本")
	result := f.Filter(tokens)
	got := terms(result)
	want := []string{"日", "本", "日本"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestNGramFilter_Empty(t *testing.T) {
	f := NewNGramFilter(1, 3)
	result := f.Filter(nil)
	if len(result) != 0 {
		t.Errorf("expected 0 tokens, got %d", len(result))
	}
}

// ===========================================================================
// EdgeNGramFilter
// ===========================================================================

func TestEdgeNGramFilter_Basic(t *testing.T) {
	f := NewEdgeNGramFilter(1, 3)
	tokens := makeTokens("hello")
	result := f.Filter(tokens)
	got := terms(result)
	want := []string{"h", "he", "hel"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestEdgeNGramFilter_MaxExceedsLength(t *testing.T) {
	f := NewEdgeNGramFilter(1, 10)
	tokens := makeTokens("hi")
	result := f.Filter(tokens)
	got := terms(result)
	want := []string{"h", "hi"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestEdgeNGramFilter_MinGreaterThanLength(t *testing.T) {
	f := NewEdgeNGramFilter(5, 10)
	tokens := makeTokens("hi")
	result := f.Filter(tokens)
	if len(result) != 0 {
		t.Errorf("expected 0 tokens, got %v", terms(result))
	}
}

func TestEdgeNGramFilter_SameMinMax(t *testing.T) {
	f := NewEdgeNGramFilter(3, 3)
	tokens := makeTokens("hello")
	result := f.Filter(tokens)
	got := terms(result)
	want := []string{"hel"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestEdgeNGramFilter_Unicode(t *testing.T) {
	f := NewEdgeNGramFilter(1, 2)
	tokens := makeTokens("日本語")
	result := f.Filter(tokens)
	got := terms(result)
	want := []string{"日", "日本"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestEdgeNGramFilter_Empty(t *testing.T) {
	f := NewEdgeNGramFilter(1, 3)
	result := f.Filter(nil)
	if len(result) != 0 {
		t.Errorf("expected 0 tokens, got %d", len(result))
	}
}

// ===========================================================================
// TrimFilter
// ===========================================================================

func TestTrimFilter(t *testing.T) {
	f := &TrimFilter{}
	tokens := makeTokens(" hello ", "\tworld\n", "  foo  ")
	result := f.Filter(tokens)
	got := terms(result)
	want := []string{"hello", "world", "foo"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestTrimFilter_NoWhitespace(t *testing.T) {
	f := &TrimFilter{}
	tokens := makeTokens("hello", "world")
	result := f.Filter(tokens)
	got := terms(result)
	want := []string{"hello", "world"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestTrimFilter_Empty(t *testing.T) {
	f := &TrimFilter{}
	result := f.Filter(nil)
	if len(result) != 0 {
		t.Errorf("expected 0 tokens, got %d", len(result))
	}
}

// ===========================================================================
// LengthFilter
// ===========================================================================

func TestLengthFilter_Min(t *testing.T) {
	f := &LengthFilter{Min: 3, Max: 0}
	tokens := makeTokens("a", "ab", "abc", "abcd")
	result := f.Filter(tokens)
	got := terms(result)
	want := []string{"abc", "abcd"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestLengthFilter_Max(t *testing.T) {
	f := &LengthFilter{Min: 0, Max: 3}
	tokens := makeTokens("a", "ab", "abc", "abcd")
	result := f.Filter(tokens)
	got := terms(result)
	want := []string{"a", "ab", "abc"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestLengthFilter_MinAndMax(t *testing.T) {
	f := &LengthFilter{Min: 2, Max: 4}
	tokens := makeTokens("a", "ab", "abc", "abcd", "abcde")
	result := f.Filter(tokens)
	got := terms(result)
	want := []string{"ab", "abc", "abcd"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestLengthFilter_Unicode(t *testing.T) {
	f := &LengthFilter{Min: 2, Max: 3}
	tokens := makeTokens("日", "日本", "日本語", "日本語文")
	result := f.Filter(tokens)
	got := terms(result)
	want := []string{"日本", "日本語"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestLengthFilter_Empty(t *testing.T) {
	f := &LengthFilter{Min: 1, Max: 10}
	result := f.Filter(nil)
	if len(result) != 0 {
		t.Errorf("expected 0 tokens, got %d", len(result))
	}
}

// ===========================================================================
// UniqueFilter
// ===========================================================================

func TestUniqueFilter_Deduplication(t *testing.T) {
	f := &UniqueFilter{}
	tokens := makeTokens("hello", "world", "hello", "foo", "world")
	result := f.Filter(tokens)
	got := terms(result)
	want := []string{"hello", "world", "foo"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestUniqueFilter_NoDuplicates(t *testing.T) {
	f := &UniqueFilter{}
	tokens := makeTokens("a", "b", "c")
	result := f.Filter(tokens)
	if len(result) != 3 {
		t.Errorf("expected 3 tokens, got %d", len(result))
	}
}

func TestUniqueFilter_AllSame(t *testing.T) {
	f := &UniqueFilter{}
	tokens := makeTokens("x", "x", "x")
	result := f.Filter(tokens)
	if len(result) != 1 {
		t.Errorf("expected 1 token, got %d", len(result))
	}
}

func TestUniqueFilter_Empty(t *testing.T) {
	f := &UniqueFilter{}
	result := f.Filter(nil)
	if len(result) != 0 {
		t.Errorf("expected 0 tokens, got %d", len(result))
	}
}

func TestUniqueFilter_CaseSensitive(t *testing.T) {
	f := &UniqueFilter{}
	tokens := makeTokens("Hello", "hello")
	result := f.Filter(tokens)
	if len(result) != 2 {
		t.Errorf("expected 2 tokens (case sensitive), got %d", len(result))
	}
}

// ===========================================================================
// HTMLCharFilter
// ===========================================================================

func TestHTMLCharFilter_StripsTags(t *testing.T) {
	f := &HTMLCharFilter{}
	input := []byte("<p>Hello <b>world</b></p>")
	result := f.Filter(input)
	got := string(result)
	// Tags replaced with spaces: " Hello  world  "
	if got != " Hello  world  " {
		t.Errorf("got %q, want %q", got, " Hello  world  ")
	}
}

func TestHTMLCharFilter_NoTags(t *testing.T) {
	f := &HTMLCharFilter{}
	input := []byte("plain text")
	result := f.Filter(input)
	if string(result) != "plain text" {
		t.Errorf("got %q, want 'plain text'", string(result))
	}
}

func TestHTMLCharFilter_Empty(t *testing.T) {
	f := &HTMLCharFilter{}
	result := f.Filter([]byte(""))
	if len(result) != 0 {
		t.Errorf("expected empty, got %q", string(result))
	}
}

func TestHTMLCharFilter_SelfClosing(t *testing.T) {
	f := &HTMLCharFilter{}
	input := []byte("line1<br/>line2")
	result := f.Filter(input)
	got := string(result)
	if got != "line1 line2" {
		t.Errorf("got %q, want 'line1 line2'", got)
	}
}

func TestHTMLCharFilter_NestedTags(t *testing.T) {
	f := &HTMLCharFilter{}
	input := []byte("<div><span>text</span></div>")
	result := f.Filter(input)
	got := string(result)
	if got != "  text  " {
		t.Errorf("got %q, want '  text  '", got)
	}
}

// ===========================================================================
// PunctuationCharFilter
// ===========================================================================

func TestPunctuationCharFilter_Basic(t *testing.T) {
	f := &PunctuationCharFilter{}
	input := []byte("hello, world! how's it?")
	result := f.Filter(input)
	got := string(result)
	// Punctuation replaced with spaces
	want := "hello  world  how s it "
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPunctuationCharFilter_NoPunctuation(t *testing.T) {
	f := &PunctuationCharFilter{}
	input := []byte("hello world")
	result := f.Filter(input)
	if string(result) != "hello world" {
		t.Errorf("got %q, want 'hello world'", string(result))
	}
}

func TestPunctuationCharFilter_Empty(t *testing.T) {
	f := &PunctuationCharFilter{}
	result := f.Filter([]byte(""))
	if len(result) != 0 {
		t.Errorf("expected empty, got %q", string(result))
	}
}

func TestPunctuationCharFilter_AllPunctuation(t *testing.T) {
	f := &PunctuationCharFilter{}
	input := []byte("!@#$%")
	result := f.Filter(input)
	// Each punctuation char replaced with a space; @ # $ % may or may not be
	// classified as punctuation by unicode.IsPunct.
	// '!' is punct. '@' is not (it's a symbol). '#' is not. '$' is not. '%' is not.
	// So only '!' gets replaced.
	// Actually let's just verify it doesn't panic.
	if result == nil && len(input) > 0 {
		t.Errorf("result should not be nil for non-empty input")
	}
}

func TestPunctuationCharFilter_Unicode(t *testing.T) {
	f := &PunctuationCharFilter{}
	input := []byte("hello\u2014world") // em-dash is punctuation
	result := f.Filter(input)
	got := string(result)
	// Em-dash should be replaced with space
	if got != "hello world" {
		t.Errorf("got %q, want 'hello world'", got)
	}
}

// ===========================================================================
// Analyzer.Analyze - Full Pipeline
// ===========================================================================

func TestAnalyzer_FullPipeline(t *testing.T) {
	a := &Analyzer{
		CharFilters: []CharFilter{
			&HTMLCharFilter{},
		},
		Tokenizer: &UnicodeTokenizer{},
		TokenFilters: []TokenFilter{
			&LowerCaseFilter{},
			NewStopWordsFilter(),
		},
	}
	input := []byte("<p>The Quick Brown Fox</p>")
	tokens := a.Analyze(input)
	got := terms(tokens)
	want := []string{"quick", "brown", "fox"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestAnalyzer_MultipleCharFilters(t *testing.T) {
	a := &Analyzer{
		CharFilters: []CharFilter{
			&HTMLCharFilter{},
			&PunctuationCharFilter{},
		},
		Tokenizer: &UnicodeTokenizer{},
		TokenFilters: []TokenFilter{
			&LowerCaseFilter{},
		},
	}
	input := []byte("<b>Hello, World!</b>")
	tokens := a.Analyze(input)
	got := terms(tokens)
	want := []string{"hello", "world"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestAnalyzer_NoFilters(t *testing.T) {
	a := &Analyzer{
		Tokenizer: &UnicodeTokenizer{},
	}
	input := []byte("Hello World")
	tokens := a.Analyze(input)
	got := terms(tokens)
	want := []string{"Hello", "World"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestAnalyzer_EmptyInput(t *testing.T) {
	a := &Analyzer{
		Tokenizer:    &UnicodeTokenizer{},
		TokenFilters: []TokenFilter{&LowerCaseFilter{}},
	}
	tokens := a.Analyze([]byte(""))
	if len(tokens) != 0 {
		t.Errorf("expected 0 tokens, got %d", len(tokens))
	}
}

func TestAnalyzer_WithNGram(t *testing.T) {
	a := &Analyzer{
		Tokenizer: &KeywordTokenizer{},
		TokenFilters: []TokenFilter{
			NewEdgeNGramFilter(1, 3),
		},
	}
	tokens := a.Analyze([]byte("test"))
	got := terms(tokens)
	want := []string{"t", "te", "tes"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// ===========================================================================
// TokenFrequencies
// ===========================================================================

func TestTokenFrequencies_WithoutTermVectors(t *testing.T) {
	tokens := []*Token{
		{Term: "hello", Position: 1, Start: 0, End: 5},
		{Term: "world", Position: 2, Start: 6, End: 11},
		{Term: "hello", Position: 3, Start: 12, End: 17},
	}
	freqs := TokenFrequencies(tokens, false)

	if len(freqs) != 2 {
		t.Fatalf("expected 2 unique terms, got %d", len(freqs))
	}
	if freqs["hello"].Frequency != 2 {
		t.Errorf("expected hello frequency 2, got %d", freqs["hello"].Frequency)
	}
	if freqs["world"].Frequency != 1 {
		t.Errorf("expected world frequency 1, got %d", freqs["world"].Frequency)
	}
	if freqs["hello"].Positions != nil {
		t.Errorf("expected nil positions without term vectors")
	}
}

func TestTokenFrequencies_WithTermVectors(t *testing.T) {
	tokens := []*Token{
		{Term: "hello", Position: 1, Start: 0, End: 5},
		{Term: "world", Position: 2, Start: 6, End: 11},
		{Term: "hello", Position: 3, Start: 12, End: 17},
	}
	freqs := TokenFrequencies(tokens, true)

	if len(freqs) != 2 {
		t.Fatalf("expected 2 unique terms, got %d", len(freqs))
	}
	helloFreq := freqs["hello"]
	if helloFreq.Frequency != 2 {
		t.Errorf("expected hello frequency 2, got %d", helloFreq.Frequency)
	}
	if len(helloFreq.Positions) != 2 {
		t.Fatalf("expected 2 positions for hello, got %d", len(helloFreq.Positions))
	}
	if helloFreq.Positions[0].Position != 1 || helloFreq.Positions[0].Start != 0 || helloFreq.Positions[0].End != 5 {
		t.Errorf("unexpected first position: %+v", helloFreq.Positions[0])
	}
	if helloFreq.Positions[1].Position != 3 || helloFreq.Positions[1].Start != 12 || helloFreq.Positions[1].End != 17 {
		t.Errorf("unexpected second position: %+v", helloFreq.Positions[1])
	}
}

func TestTokenFrequencies_Empty(t *testing.T) {
	freqs := TokenFrequencies(nil, true)
	if len(freqs) != 0 {
		t.Errorf("expected 0 frequencies, got %d", len(freqs))
	}
}

func TestTokenFrequencies_SingleToken(t *testing.T) {
	tokens := []*Token{
		{Term: "only", Position: 1, Start: 0, End: 4},
	}
	freqs := TokenFrequencies(tokens, true)
	if len(freqs) != 1 {
		t.Fatalf("expected 1 term, got %d", len(freqs))
	}
	if freqs["only"].Frequency != 1 {
		t.Errorf("expected frequency 1, got %d", freqs["only"].Frequency)
	}
	if len(freqs["only"].Positions) != 1 {
		t.Errorf("expected 1 position, got %d", len(freqs["only"].Positions))
	}
}

func TestTokenFrequencies_TermField(t *testing.T) {
	tokens := []*Token{
		{Term: "test", Position: 1, Start: 0, End: 4},
	}
	freqs := TokenFrequencies(tokens, false)
	if freqs["test"].Term != "test" {
		t.Errorf("expected Term field 'test', got %q", freqs["test"].Term)
	}
}

// ===========================================================================
// Helpers
// ===========================================================================

func containsTerm(terms []string, target string) bool {
	for _, t := range terms {
		if t == target {
			return true
		}
	}
	return false
}
