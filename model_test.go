package psd

import (
	"testing"
)

func TestItemGetDomain_WithDomain(t *testing.T) {
	item := &Item{
		Targets: []string{"t1"},
		Labels:  map[string]string{"domain": "example.com", "other": "x"},
	}

	got := item.GetDomain()
	if got != "example.com" {
		t.Fatalf("GetDomain() = %q, want %q", got, "example.com")
	}
}

func TestItemGetDomain_NoDomainKey(t *testing.T) {
	item := &Item{
		Targets: []string{"t1"},
		Labels:  map[string]string{"other": "x"},
	}

	got := item.GetDomain()
	if got != "" {
		t.Fatalf("GetDomain() = %q, want empty string", got)
	}
}

func TestItemGetDomain_NilLabels(t *testing.T) {
	item := &Item{
		Targets: []string{"t1"},
		Labels:  nil,
	}

	got := item.GetDomain()
	if got != "" {
		t.Fatalf("GetDomain() = %q, want empty string for nil Labels", got)
	}
}

func TestItemsGetDomains_EmptySlice(t *testing.T) {
	var items Items

	got := items.GetDomains()
	if got == nil {
		t.Fatalf("GetDomains() returned nil, want empty slice")
	}
	if len(got) != 0 {
		t.Fatalf("GetDomains() len = %d, want 0", len(got))
	}
}

func TestItemsGetDomains_NoDomains(t *testing.T) {
	items := Items{
		&Item{Labels: map[string]string{}},
		&Item{Labels: map[string]string{"other": "x"}},
		&Item{Labels: nil},
	}

	got := items.GetDomains()
	if len(got) != 0 {
		t.Fatalf("GetDomains() len = %d, want 0", len(got))
	}
}

func TestItemsGetDomains_DeduplicatesAndIgnoresEmpty(t *testing.T) {
	items := Items{
		&Item{Labels: map[string]string{"domain": "example.com"}},
		&Item{Labels: map[string]string{"domain": "example.com"}}, // duplicate
		&Item{Labels: map[string]string{"domain": "example.org"}},
		&Item{Labels: map[string]string{"domain": ""}},             // empty should be ignored
		&Item{Labels: map[string]string{"other": "no-domain-key"}}, // missing key ignored
		nil, // nil *Item should panic if dereferenced; ensure we don't have such entries in normal use
	}

	// Remove nil to avoid panic; also explicitly test robustness without nil first.
	itemsNoNil := items[:len(items)-1]

	got := itemsNoNil.GetDomains()
	if len(got) != 2 {
		t.Fatalf("GetDomains() len = %d, want 2 (example.com, example.org)", len(got))
	}

	// Order is not guaranteed, so check presence via map
	gotMap := make(map[string]bool, len(got))
	for _, d := range got {
		gotMap[d] = true
	}

	if !gotMap["example.com"] {
		t.Fatalf("GetDomains() missing domain %q", "example.com")
	}
	if !gotMap["example.org"] {
		t.Fatalf("GetDomains() missing domain %q", "example.org")
	}
}

func TestItemsContains_Found(t *testing.T) {
	items := Items{
		&Item{Targets: []string{"a", "b", "c"}},
		&Item{Targets: []string{"d", "e"}},
	}

	if !items.Contains("b") {
		t.Fatalf("Contains() = false, want true for existing target")
	}
	if !items.Contains("e") {
		t.Fatalf("Contains() = false, want true for existing target in later item")
	}
}

func TestItemsContains_NotFound(t *testing.T) {
	items := Items{
		&Item{Targets: []string{"a", "b"}},
	}

	if items.Contains("x") {
		t.Fatalf("Contains() = true, want false for non-existing target")
	}
}

func TestItemsContains_EmptyItems(t *testing.T) {
	var items Items

	if items.Contains("anything") {
		t.Fatalf("Contains() = true, want false for empty Items")
	}
}

func TestItemsContains_EmptyTargetsAndNeedle(t *testing.T) {
	items := Items{
		&Item{Targets: nil},
		&Item{Targets: []string{}},
	}

	// slices.Contains on empty or nil should be false
	if items.Contains("") {
		t.Fatalf("Contains() = true, want false for empty needle with no targets")
	}
}

func TestItemsContainsSuffix_Found(t *testing.T) {
	items := Items{
		&Item{Targets: []string{"http://example.com/path", "https://example.org/other"}},
		&Item{Targets: []string{"ftp://example.net/file.txt"}},
	}

	if !items.ContainsSuffix("path") {
		t.Fatalf("ContainsSuffix() = false, want true for suffix 'path'")
	}
	if !items.ContainsSuffix(".txt") {
		t.Fatalf("ContainsSuffix() = false, want true for suffix '.txt'")
	}
}

func TestItemsContainsSuffix_NotFound(t *testing.T) {
	items := Items{
		&Item{Targets: []string{"abc", "def"}},
	}

	if items.ContainsSuffix("xyz") {
		t.Fatalf("ContainsSuffix() = true, want false for unmatched suffix")
	}
}

func TestItemsContainsSuffix_EmptyNeedle(t *testing.T) {
	items := Items{
		&Item{Targets: []string{"a", ""}},
	}

	// strings.HasSuffix(x, "") is true for all strings, including ""
	if !items.ContainsSuffix("") {
		t.Fatalf("ContainsSuffix(\"\") = false, want true because any string has empty suffix")
	}
}

func TestItemsContainsSuffix_EmptyItems(t *testing.T) {
	var items Items
	if items.ContainsSuffix("anything") {
		t.Fatalf("ContainsSuffix() = true, want false for empty Items")
	}
}

func TestItemsContainsDiscovery_Positive(t *testing.T) {
	suffix := "/.well-known/matrix/client"
	items := Items{
		&Item{Targets: []string{"https://example.com" + suffix}},
	}

	if !items.ContainsDiscovery() {
		t.Fatalf("ContainsDiscovery() = false, want true when target ends with %q", suffix)
	}
}

func TestItemsContainsDiscovery_Negative(t *testing.T) {
	suffix := "/.well-known/matrix/client"
	items := Items{
		&Item{Targets: []string{"https://example.com/.well-known/matrix/other"}},
	}

	if items.ContainsDiscovery() {
		t.Fatalf("ContainsDiscovery() = true, want false when no target ends with %q", suffix)
	}
}

func TestItemsContainsFederation_Positive(t *testing.T) {
	suffix := "/_matrix/federation/v1/version"
	items := Items{
		&Item{Targets: []string{"https://matrix.example.com" + suffix}},
	}

	if !items.ContainsFederation() {
		t.Fatalf("ContainsFederation() = false, want true when target ends with %q", suffix)
	}
}

func TestItemsContainsFederation_Negative(t *testing.T) {
	suffix := "/_matrix/federation/v1/version"
	items := Items{
		&Item{Targets: []string{"https://matrix.example.com/_matrix/federation/v1/other"}},
	}

	if items.ContainsFederation() {
		t.Fatalf("ContainsFederation() = true, want false when no target ends with %q", suffix)
	}
}

func TestItemsContainsDelegation_Positive(t *testing.T) {
	suffix := "/.well-known/matrix/server"
	items := Items{
		&Item{Targets: []string{"https://example.com" + suffix}},
	}

	if !items.ContainsDelegation() {
		t.Fatalf("ContainsDelegation() = false, want true when target ends with %q", suffix)
	}
}

func TestItemsContainsDelegation_Negative(t *testing.T) {
	suffix := "/.well-known/matrix/server"
	items := Items{
		&Item{Targets: []string{"https://example.com/.well-known/matrix/other"}},
	}

	if items.ContainsDelegation() {
		t.Fatalf("ContainsDelegation() = true, want false when no target ends with %q", suffix)
	}
}

func TestItemsContainsMSC1929_Positive(t *testing.T) {
	suffix := "/.well-known/matrix/support"
	items := Items{
		&Item{Targets: []string{"https://support.example.com" + suffix}},
	}

	if !items.ContainsMSC1929() {
		t.Fatalf("ContainsMSC1929() = false, want true when target ends with %q", suffix)
	}
}

func TestItemsContainsMSC1929_Negative(t *testing.T) {
	suffix := "/.well-known/matrix/support"
	items := Items{
		&Item{Targets: []string{"https://support.example.com/.well-known/matrix/other"}},
	}

	if items.ContainsMSC1929() {
		t.Fatalf("ContainsMSC1929() = true, want false when no target ends with %q", suffix)
	}
}
