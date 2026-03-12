package analysis

import "testing"

var mediumText = []byte(`The quick brown fox jumps over the lazy dog.
This is a medium-length paragraph designed to benchmark the Unicode tokenizer.
It contains multiple sentences with various punctuation marks, numbers like 42 and 3.14,
and some unicode characters: cafe, naive, uber. The text should be long enough to
exercise the tokenizer but not so long that the benchmark takes forever. Here are some
more words to pad it out: algorithm, benchmark, compiler, database, encryption,
framework, gateway, hashmap, iterator, javascript, kubernetes, lambda, middleware,
namespace, orchestration, pipeline, query, repository, scheduler, terraform.`)

var goSource = []byte(`package main

import (
	"fmt"
	"strings"
)

// SearchIndex performs full-text search across documents.
type SearchIndex struct {
	store   Store
	mapping *IndexMapping
}

func NewSearchIndex(store Store) *SearchIndex {
	return &SearchIndex{store: store, mapping: NewIndexMapping()}
}

func (si *SearchIndex) IndexDocument(id string, data map[string]interface{}) error {
	doc := NewDocument(id)
	for fieldName, fieldValue := range data {
		if strVal, ok := fieldValue.(string); ok {
			doc.AddField(fieldName, strings.ToLower(strVal))
		}
	}
	return si.store.Set(fmt.Sprintf("doc:%s", id), doc.Serialize())
}
`)

func BenchmarkTokenize_Unicode(b *testing.B) {
	tok := &UnicodeTokenizer{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tok.Tokenize(mediumText)
	}
}

func BenchmarkTokenize_Code(b *testing.B) {
	tok := &CodeTokenizer{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tok.Tokenize(goSource)
	}
}

func BenchmarkAnalyze_Standard(b *testing.B) {
	a := &Analyzer{
		Tokenizer: &UnicodeTokenizer{},
		TokenFilters: []TokenFilter{
			&LowerCaseFilter{},
			NewStopWordsFilter(),
		},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a.Analyze(mediumText)
	}
}
