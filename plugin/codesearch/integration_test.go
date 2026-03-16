package codesearch_test

import (
	"context"
	"testing"

	"github.com/readmedotmd/store.md/backend/memory"

	searchmd "github.com/readmedotmd/search.md"
	"github.com/readmedotmd/search.md/mapping"
	"github.com/readmedotmd/search.md/plugin/codesearch"
	"github.com/readmedotmd/search.md/search/query"
)

func newMemStore() *memory.StoreMemory {
	return memory.New()
}

func TestIntegration_SymbolFieldIndexer(t *testing.T) {
	store := newMemStore()

	// Create mapping with a symbol field.
	im := mapping.NewIndexMapping()
	dm := mapping.NewDocumentStaticMapping()
	dm.AddFieldMapping("source", mapping.NewSymbolFieldMapping("go"))
	dm.AddFieldMapping("filename", mapping.NewKeywordFieldMapping())
	im.DefaultMapping = dm

	// Create index with the regex symbol indexer.
	idx, err := searchmd.New(store, im, codesearch.WithRegexSymbolIndexer())
	if err != nil {
		t.Fatal(err)
	}

	// Index some Go source files.
	err = idx.Index("file1", map[string]interface{}{
		"filename": "user.go",
		"source": `package models

type User struct {
	ID   int
	Name string
	Email string
}

func NewUser(name, email string) *User {
	return &User{Name: name, Email: email}
}

func (u *User) Validate() error {
	if u.Name == "" {
		return fmt.Errorf("name required")
	}
	return nil
}
`,
	})
	if err != nil {
		t.Fatal(err)
	}

	err = idx.Index("file2", map[string]interface{}{
		"filename": "service.go",
		"source": `package service

type UserService struct {
	repo UserRepository
}

type UserRepository interface {
	FindByID(id int) (*User, error)
	Save(user *User) error
}

func NewUserService(repo UserRepository) *UserService {
	return &UserService{repo: repo}
}

func (s *UserService) GetUser(id int) (*User, error) {
	return s.repo.FindByID(id)
}
`,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Search for symbol names.
	t.Run("search by symbol name", func(t *testing.T) {
		results, err := idx.Search(context.Background(), query.NewTermQuery("newuser").SetField("source.sym"))
		if err != nil {
			t.Fatal(err)
		}
		if results.Total == 0 {
			t.Error("expected results for 'newuser' symbol search")
		}
		found := false
		for _, hit := range results.Hits {
			if hit.ID == "file1" {
				found = true
			}
		}
		if !found {
			t.Error("expected file1 in results")
		}
	})

	// Search by symbol kind.
	t.Run("search by symbol kind", func(t *testing.T) {
		results, err := idx.Search(context.Background(), query.NewTermQuery("interface").SetField("source.kind"))
		if err != nil {
			t.Fatal(err)
		}
		if results.Total == 0 {
			t.Error("expected results for 'interface' kind search")
		}
		found := false
		for _, hit := range results.Hits {
			if hit.ID == "file2" {
				found = true
			}
		}
		if !found {
			t.Error("expected file2 in results for interface search")
		}
	})

	// Search for struct kind.
	t.Run("search for structs", func(t *testing.T) {
		results, err := idx.Search(context.Background(), query.NewTermQuery("struct").SetField("source.kind"))
		if err != nil {
			t.Fatal(err)
		}
		if results.Total < 2 {
			t.Errorf("expected at least 2 results for struct kind, got %d", results.Total)
		}
	})

	// Search for functions across files.
	t.Run("search function kind", func(t *testing.T) {
		results, err := idx.Search(context.Background(), query.NewTermQuery("function").SetField("source.kind"))
		if err != nil {
			t.Fatal(err)
		}
		if results.Total < 2 {
			t.Errorf("expected at least 2 files with functions, got %d", results.Total)
		}
	})

	// Verify deletion works.
	t.Run("delete document", func(t *testing.T) {
		if err := idx.Delete("file1"); err != nil {
			t.Fatal(err)
		}
		results, err := idx.Search(context.Background(), query.NewTermQuery("newuser").SetField("source.sym"))
		if err != nil {
			t.Fatal(err)
		}
		if results.Total != 0 {
			t.Errorf("expected 0 results after deletion, got %d", results.Total)
		}
	})

	// Verify re-indexing works.
	t.Run("re-index document", func(t *testing.T) {
		err := idx.Index("file1", map[string]interface{}{
			"filename": "user.go",
			"source": `package models

func UpdateUser(id int, name string) error {
	return nil
}
`,
		})
		if err != nil {
			t.Fatal(err)
		}

		// Old symbol should be gone.
		results, err := idx.Search(context.Background(), query.NewTermQuery("newuser").SetField("source.sym"))
		if err != nil {
			t.Fatal(err)
		}
		if results.Total != 0 {
			t.Errorf("expected 0 results for old symbol, got %d", results.Total)
		}

		// New symbol should be found.
		results, err = idx.Search(context.Background(), query.NewTermQuery("updateuser").SetField("source.sym"))
		if err != nil {
			t.Fatal(err)
		}
		if results.Total == 0 {
			t.Error("expected results for new symbol 'updateuser'")
		}
	})
}

func TestIntegration_TagExtractor(t *testing.T) {
	store := newMemStore()

	im := mapping.NewIndexMapping()
	dm := mapping.NewDocumentStaticMapping()
	dm.AddFieldMapping("code", mapping.NewSymbolFieldMapping("python"))
	im.DefaultMapping = dm

	idx, err := searchmd.New(store, im, codesearch.WithTagIndexer())
	if err != nil {
		t.Fatal(err)
	}

	err = idx.Index("app.py", map[string]interface{}{
		"code": `class Application:
    def __init__(self, config):
        self.config = config

    def run(self):
        print("running")

def create_app(config):
    return Application(config)
`,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Search for class.
	results, err := idx.Search(context.Background(), query.NewTermQuery("application").SetField("code.sym"))
	if err != nil {
		t.Fatal(err)
	}
	if results.Total == 0 {
		t.Error("expected results for 'application' symbol")
	}

	// Search for method kind.
	results, err = idx.Search(context.Background(), query.NewTermQuery("method").SetField("code.kind"))
	if err != nil {
		t.Fatal(err)
	}
	if results.Total == 0 {
		t.Error("expected results for 'method' kind")
	}
}

func TestIntegration_TreeSitterExtractor(t *testing.T) {
	store := newMemStore()

	// Create a mock tree-sitter parser.
	mockRoot := &codesearch.ASTNode{
		Type: "source_file",
		Children: []*codesearch.ASTNode{
			{
				Type: "function_declaration",
				Children: []*codesearch.ASTNode{
					{Type: "identifier", Content: "ProcessData", FieldName: "name"},
				},
				StartRow: 0,
			},
			{
				Type: "type_spec",
				Children: []*codesearch.ASTNode{
					{Type: "type_identifier", Content: "DataProcessor", FieldName: "name"},
				},
				StartRow: 5,
			},
		},
	}

	tsExtractor := codesearch.NewTreeSitterExtractor()
	tsExtractor.AddLanguage("go", &mockASTParser{root: mockRoot}, nil)

	im := mapping.NewIndexMapping()
	dm := mapping.NewDocumentStaticMapping()
	dm.AddFieldMapping("code", mapping.NewSymbolFieldMapping("go"))
	im.DefaultMapping = dm

	idx, err := searchmd.New(store, im, codesearch.WithTreeSitterIndexer(tsExtractor))
	if err != nil {
		t.Fatal(err)
	}

	err = idx.Index("proc.go", map[string]interface{}{
		"code": "func ProcessData() {}\ntype DataProcessor struct{}",
	})
	if err != nil {
		t.Fatal(err)
	}

	results, err := idx.Search(context.Background(), query.NewTermQuery("processdata").SetField("code.sym"))
	if err != nil {
		t.Fatal(err)
	}
	if results.Total == 0 {
		t.Error("expected results for 'processdata' symbol from tree-sitter")
	}

	results, err = idx.Search(context.Background(), query.NewTermQuery("dataprocessor").SetField("code.sym"))
	if err != nil {
		t.Fatal(err)
	}
	if results.Total == 0 {
		t.Error("expected results for 'dataprocessor' symbol from tree-sitter")
	}
}

// mockASTParser implements codesearch.ASTParser for testing.
type mockASTParser struct {
	root *codesearch.ASTNode
}

func (p *mockASTParser) Parse(source []byte) (*codesearch.ASTNode, error) {
	return p.root, nil
}
