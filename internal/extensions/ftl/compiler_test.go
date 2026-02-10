package ftl

import (
	"testing"
)

func TestTokenize(t *testing.T) {
	c := NewCompiler(DefaultCompilerConfig())
	tokens, err := c.Tokenize("SELECT col1, col2 FROM features WHERE col1 > 10")
	if err != nil {
		t.Fatal(err)
	}
	// SELECT, col1, COMMA, col2, FROM, features, WHERE, col1, >, 10, EOF = 11
	if len(tokens) != 11 {
		t.Fatalf("expected 11 tokens, got %d: %+v", len(tokens), tokens)
	}
	if tokens[0].Type != TokenSelect {
		t.Errorf("expected TokenSelect, got %d", tokens[0].Type)
	}
	if tokens[4].Type != TokenFrom {
		t.Errorf("expected TokenFrom, got %d (%q)", tokens[4].Type, tokens[4].Value)
	}
	if tokens[6].Type != TokenWhere {
		t.Errorf("expected TokenWhere, got %d (%q)", tokens[6].Type, tokens[6].Value)
	}
}

func TestParse(t *testing.T) {
	c := NewCompiler(DefaultCompilerConfig())
	ast, err := c.Parse("SELECT col1, col2 FROM features WHERE col1 > 10")
	if err != nil {
		t.Fatal(err)
	}
	if ast.Type != NodeTypeSelect {
		t.Errorf("expected select node, got %s", ast.Type)
	}
	// Children: col1 (column_ref), col2 (column_ref), from, where
	if len(ast.Children) != 4 {
		t.Fatalf("expected 4 children, got %d", len(ast.Children))
	}
	if ast.Children[0].Type != NodeTypeColumnRef || ast.Children[0].Value != "col1" {
		t.Errorf("expected column_ref col1, got %s %q", ast.Children[0].Type, ast.Children[0].Value)
	}
	if ast.Children[2].Type != NodeTypeFrom {
		t.Errorf("expected from node, got %s", ast.Children[2].Type)
	}
	if ast.Children[3].Type != NodeTypeWhere {
		t.Errorf("expected where node, got %s", ast.Children[3].Type)
	}
}

func TestCompileAndExecute(t *testing.T) {
	c := NewCompiler(DefaultCompilerConfig())

	p, err := c.Compile("test-pipeline", "SELECT col1, col2 FROM features WHERE col1 > 10")
	if err != nil {
		t.Fatal(err)
	}
	if !p.Compiled {
		t.Error("expected pipeline to be compiled")
	}

	data := []map[string]interface{}{
		{"col1": 5, "col2": "a"},
		{"col1": 15, "col2": "b"},
		{"col1": 20, "col2": "c"},
	}

	result, err := c.Execute(p.ID, data)
	if err != nil {
		t.Fatal(err)
	}
	if result.RowCount != 2 {
		t.Errorf("expected 2 rows, got %d", result.RowCount)
	}
}

func TestExecuteQuery(t *testing.T) {
	c := NewCompiler(DefaultCompilerConfig())

	data := []map[string]interface{}{
		{"col1": 5, "col2": "a"},
		{"col1": 15, "col2": "b"},
		{"col1": 20, "col2": "c"},
	}

	result, err := c.ExecuteQuery("SELECT col2 FROM features WHERE col1 > 10", data)
	if err != nil {
		t.Fatal(err)
	}
	if result.RowCount != 2 {
		t.Errorf("expected 2 rows, got %d", result.RowCount)
	}
	if len(result.Columns) != 1 || result.Columns[0] != "col2" {
		t.Errorf("expected [col2], got %v", result.Columns)
	}
}

func TestListAndDeletePipeline(t *testing.T) {
	c := NewCompiler(DefaultCompilerConfig())

	p, err := c.Compile("p1", "SELECT col1 FROM t")
	if err != nil {
		t.Fatal(err)
	}

	pipelines := c.ListPipelines()
	if len(pipelines) != 1 {
		t.Fatalf("expected 1 pipeline, got %d", len(pipelines))
	}

	if err := c.DeletePipeline(p.ID); err != nil {
		t.Fatal(err)
	}

	pipelines = c.ListPipelines()
	if len(pipelines) != 0 {
		t.Fatalf("expected 0 pipelines, got %d", len(pipelines))
	}
}

func TestStats(t *testing.T) {
	c := NewCompiler(DefaultCompilerConfig())

	c.Compile("s1", "SELECT col1 FROM t")
	stats := c.Stats()
	if stats.PipelinesCompiled != 1 {
		t.Errorf("expected 1 compiled, got %d", stats.PipelinesCompiled)
	}
}

func TestSelectStar(t *testing.T) {
	c := NewCompiler(DefaultCompilerConfig())
	data := []map[string]interface{}{
		{"a": 1, "b": 2},
	}
	result, err := c.ExecuteQuery("SELECT * FROM t", data)
	if err != nil {
		t.Fatal(err)
	}
	if result.RowCount != 1 {
		t.Errorf("expected 1 row, got %d", result.RowCount)
	}
}
