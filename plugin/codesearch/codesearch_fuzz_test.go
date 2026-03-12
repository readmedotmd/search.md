package codesearch

import (
	"testing"
)

func FuzzRegexExtractor(f *testing.F) {
	f.Add([]byte("func main() {\n\tfmt.Println(\"hello\")\n}"), "go")
	f.Add([]byte("class Foo:\n    def bar(self):\n        pass"), "python")
	f.Add([]byte("export function hello() {}"), "javascript")
	f.Add([]byte(""), "go")
	f.Add([]byte("\x00\xff\xfe"), "")
	f.Add([]byte("pub fn main() {}"), "rust")
	f.Add([]byte("struct Node {\n    int val;\n};"), "c")

	extractor := NewRegexExtractor()

	languages := []string{"go", "python", "javascript", "typescript", "java", "rust", "c", "c++", "ruby", "php", "unknown"}

	f.Fuzz(func(t *testing.T, source []byte, lang string) {
		// Must not panic on any input.
		_, _ = extractor.Extract(source, lang)

		// Also test with each known language to cover all pattern sets.
		for _, l := range languages {
			_, _ = extractor.Extract(source, l)
		}
	})
}

func FuzzParseTags(f *testing.F) {
	f.Add("main\tmain.go\t1;\"\tf")
	f.Add("Foo\tfoo.go\t10;\"\ts\tscope:main")
	f.Add("!_TAG_FILE_FORMAT\t2\t/extended format/\n!_TAG_FILE_SORTED\t1\t/0=unsorted/\nmain\tmain.go\t1;\"\tf")
	f.Add("")
	f.Add("\t\t\t")
	f.Add("name\tfile\t99;\"\tm\tscope:MyClass\tsignature:(int x)")
	f.Add("\x00\xff\xfe")

	f.Fuzz(func(t *testing.T, input string) {
		// Must not panic on any input.
		_, _ = ParseTags(input)
	})
}
