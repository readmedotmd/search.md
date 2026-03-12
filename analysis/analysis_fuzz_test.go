package analysis

import (
	"testing"
)

func FuzzUnicodeTokenizer(f *testing.F) {
	f.Add([]byte("hello world"))
	f.Add([]byte(""))
	f.Add([]byte("one"))
	f.Add([]byte("CamelCase snake_case kebab-case"))
	f.Add([]byte("\x00\xff\xfe"))
	f.Add([]byte("日本語テスト"))
	f.Add([]byte("a\tb\nc\rd"))

	tokenizer := &UnicodeTokenizer{}

	f.Fuzz(func(t *testing.T, data []byte) {
		// Must not panic on any input.
		_ = tokenizer.Tokenize(data)
	})
}

func FuzzCodeTokenizer(f *testing.F) {
	f.Add([]byte("func main() {}"))
	f.Add([]byte(""))
	f.Add([]byte("camelCaseFunction"))
	f.Add([]byte("snake_case_var"))
	f.Add([]byte("pkg.Method"))
	f.Add([]byte("a.b.c.d_e_f"))
	f.Add([]byte("日本語.テスト_変数"))
	f.Add([]byte("===!!!@@@###"))

	tokenizer := &CodeTokenizer{}

	f.Fuzz(func(t *testing.T, data []byte) {
		// Must not panic on any input.
		_ = tokenizer.Tokenize(data)
	})
}
