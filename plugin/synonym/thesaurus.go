// Package synonym provides a synonym expansion token filter backed by the
// Moby Thesaurus II, a public-domain English thesaurus containing over 30,000
// root words and 2.5 million synonyms.
//
// The thesaurus data is embedded in the binary (gzip-compressed) and decoded
// on first use. Subsequent lookups use an in-memory map.
//
// License: The Moby Thesaurus data is in the public domain.
// "The Moby lexicon project is complete and has been placed into the
// public domain. Use, sell, rework, excerpt and use in any way on any
// platform." — Grady Ward
// Source: https://github.com/words/moby
package synonym

import (
	"bufio"
	"compress/gzip"
	"bytes"
	_ "embed"
	"strings"
	"sync"
)

//go:embed thesaurus.gz
var thesaurusGZ []byte

// thesaurus is the lazily-initialized synonym map.
// Keys are lowercase root words; values are their synonym lists.
var (
	thesaurusOnce sync.Once
	thesaurusMap  map[string][]string
)

// loadThesaurus decompresses and parses the embedded thesaurus data.
// Each line is: rootword,syn1,syn2,...
func loadThesaurus() {
	thesaurusOnce.Do(func() {
		thesaurusMap = make(map[string][]string, 32000)

		gz, err := gzip.NewReader(bytes.NewReader(thesaurusGZ))
		if err != nil {
			return
		}
		defer gz.Close()

		scanner := bufio.NewScanner(gz)
		// Some lines in the Moby thesaurus are very long.
		scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024)

		for scanner.Scan() {
			line := scanner.Text()
			if len(line) == 0 {
				continue
			}
			parts := strings.Split(line, ",")
			if len(parts) < 2 {
				continue
			}
			key := strings.ToLower(strings.TrimSpace(parts[0]))
			syns := make([]string, 0, len(parts)-1)
			for _, s := range parts[1:] {
				s = strings.TrimSpace(s)
				if s != "" {
					syns = append(syns, strings.ToLower(s))
				}
			}
			if len(syns) > 0 {
				thesaurusMap[key] = syns
			}
		}
	})
}

// Lookup returns the synonyms for a word, or nil if not found.
// The word is matched case-insensitively.
func Lookup(word string) []string {
	loadThesaurus()
	return thesaurusMap[strings.ToLower(word)]
}
