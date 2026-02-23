package sdkcodegen

import (
	"fmt"
	"sort"
	"sync"
)

// Language values.
const (
	LangJava   Language = "java"
	LangRust   Language = "rust"
	LangSwift  Language = "swift"
	LangKotlin Language = "kotlin"
)

// LanguageRegistry maps language names to their code generators.
type LanguageRegistry struct {
	mu         sync.RWMutex
	generators map[Language]func(*SchemaDefinition) string
}

// NewLanguageRegistry creates a registry pre-loaded with all supported languages.
func NewLanguageRegistry() *LanguageRegistry {
	r := &LanguageRegistry{
		generators: make(map[Language]func(*SchemaDefinition) string),
	}

	g := NewGenerator(DefaultGeneratorConfig())

	// Register built-in languages
	r.Register(LangGo, func(s *SchemaDefinition) string {
		code, _, _ := g.generateGo(s)
		return code
	})
	r.Register(LangPython, func(s *SchemaDefinition) string {
		code, _, _ := g.generatePython(s)
		return code
	})
	r.Register(LangTypeScript, func(s *SchemaDefinition) string {
		code, _, _ := g.generateTypeScript(s)
		return code
	})
	r.Register(LangJava, func(s *SchemaDefinition) string {
		code, _, _ := g.generateJava(s)
		return code
	})
	r.Register(LangRust, func(s *SchemaDefinition) string {
		code, _, _ := g.generateRust(s)
		return code
	})
	r.Register(LangSwift, func(s *SchemaDefinition) string {
		code, _, _ := g.generateSwift(s)
		return code
	})
	r.Register(LangKotlin, func(s *SchemaDefinition) string {
		code, _, _ := g.generateKotlin(s)
		return code
	})

	return r
}

// Register adds a language generator to the registry.
func (r *LanguageRegistry) Register(lang Language, gen func(*SchemaDefinition) string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.generators[lang] = gen
}

// Generate produces code for the given language and schema.
func (r *LanguageRegistry) Generate(lang Language, schema *SchemaDefinition) (string, error) {
	r.mu.RLock()
	gen, ok := r.generators[lang]
	r.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("%w: %s", ErrUnsupportedLanguage, lang)
	}
	return gen(schema), nil
}

// SupportedLanguages returns a sorted list of all registered languages.
func (r *LanguageRegistry) SupportedLanguages() []Language {
	r.mu.RLock()
	defer r.mu.RUnlock()

	langs := make([]Language, 0, len(r.generators))
	for lang := range r.generators {
		langs = append(langs, lang)
	}
	sort.Slice(langs, func(i, j int) bool {
		return langs[i] < langs[j]
	})
	return langs
}
