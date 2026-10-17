package core

import (
	"reflect"
	"testing"
)

func TestDefaultBindingsSurviveDefinitionReorder(t *testing.T) {
	original := clientDefinitions
	reordered := append([]clientDefinition(nil), original...)
	reordered[0], reordered[1] = reordered[1], reordered[0]
	clientDefinitions = reordered
	defer func() { clientDefinitions = original }()
	registry := defaultClients(&Integrator{Home: t.TempDir()})
	assertClientSelections(t, registry)
}
func assertClientSelections(t *testing.T, r *ClientRegistry) {
	t.Helper()
	for _, tc := range []struct {
		id, name string
		profile  Connection
	}{
		{"codex", "Codex", Connection{Codex: true}},
		{"chatgpt", "ChatGPT Desktop", Connection{ChatGPT: true}},
		{"claude", "Claude Code", Connection{Claude: true}},
		{"claude-desktop", "Claude Desktop", Connection{ClaudeDesktop: true}},
	} {
		binding, err := r.Lookup(tc.id)
		if err != nil {
			t.Fatal(err)
		}
		if binding.Name != tc.name || !binding.Selected(tc.profile) {
			t.Fatalf("%s misbound: name=%q, selected=%v", tc.id, binding.Name, binding.Selected(tc.profile))
		}
		for _, other := range r.IDs() {
			if other == tc.id {
				continue
			}
			b, _ := r.Lookup(other)
			if b.Selected(tc.profile) {
				t.Fatalf("%s-only choice selected %s", tc.id, other)
			}
		}
	}
}
func TestBindingOrderIndependentOfAdapterOrder(t *testing.T) {
	host := &Integrator{Home: t.TempDir()}
	adapters := []ClientAdapter{NewClaudeDesktopAdapter(host, ""), NewClaudeAdapter(host, ""), NewChatGPTAdapter(host, ""), NewCodexAdapter(host, "")}
	r, err := bindClientAdapters(clientDefinitions, adapters)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(r.IDs(), ClientIDs) {
		t.Fatal("report order changed", r.IDs())
	}
	assertClientSelections(t, r)
}
func TestBindingRejectsInvalidComposition(t *testing.T) {
	host := &Integrator{Home: t.TempDir()}
	a := NewCodexAdapter(host, "")
	d := clientDefinition{"codex", "Codex", func(c Connection) bool { return c.Codex }}
	for _, tc := range []struct {
		name        string
		definitions []clientDefinition
		adapters    []ClientAdapter
	}{
		{"missing", []clientDefinition{d}, nil},
		{"unknown", nil, []ClientAdapter{a}},
		{"duplicate-adapter", []clientDefinition{d}, []ClientAdapter{a, a}},
		{"duplicate-definition", []clientDefinition{d, d}, []ClientAdapter{a}},
		{"nil-adapter", []clientDefinition{d}, []ClientAdapter{nil}},
		{"invalid-definition", []clientDefinition{{id: "codex"}}, []ClientAdapter{a}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := bindClientAdapters(tc.definitions, tc.adapters); err == nil {
				t.Fatal("invalid composition accepted")
			}
		})
	}
}
