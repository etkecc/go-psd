package psd

import (
	"testing"
)

func TestItem_GetDomain(t *testing.T) {
	tests := []struct {
		name string
		item Item
		want string
	}{
		{
			name: "returns domain when present",
			item: Item{
				Labels: map[string]string{"domain": "example.com"},
			},
			want: "example.com",
		},
		{
			name: "returns empty string when domain is missing",
			item: Item{
				Labels: map[string]string{"other": "value"},
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.item.GetDomain(); got != tt.want {
				t.Errorf("GetDomain() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestItems_GetDomains(t *testing.T) {
	tests := []struct {
		name  string
		items Items
		want  []string
	}{
		{
			name: "returns unique domains from items",
			items: Items{
				&Item{Labels: map[string]string{"domain": "example.com"}},
				&Item{Labels: map[string]string{"domain": "example.org"}},
				&Item{Labels: map[string]string{"domain": "example.com"}},
			},
			want: []string{"example.com", "example.org"},
		},
		{
			name:  "returns empty slice when no items",
			items: Items{},
			want:  []string{},
		},
		{
			name: "returns empty slice when no domains",
			items: Items{
				&Item{Labels: map[string]string{"other": "value"}},
			},
			want: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.items.GetDomains()
			if len(got) != len(tt.want) {
				t.Errorf("GetDomains() = %v, want %v", got, tt.want)
			}
			domainMap := map[string]bool{}
			for _, domain := range got {
				domainMap[domain] = true
			}
			for _, wantDomain := range tt.want {
				if !domainMap[wantDomain] {
					t.Errorf("GetDomains() = %v, missing %v", got, wantDomain)
				}
			}
		})
	}
}

func TestItems_Contains(t *testing.T) {
	tests := []struct {
		name   string
		items  Items
		needle string
		want   bool
	}{
		{
			name: "returns true when target exists",
			items: Items{
				&Item{Targets: []string{"target1", "target2"}},
			},
			needle: "target1",
			want:   true,
		},
		{
			name: "returns false when target does not exist",
			items: Items{
				&Item{Targets: []string{"target1", "target2"}},
			},
			needle: "target3",
			want:   false,
		},
		{
			name:   "returns false when no items",
			items:  Items{},
			needle: "target1",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.items.Contains(tt.needle); got != tt.want {
				t.Errorf("Contains() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestItems_ContainsFederation(t *testing.T) {
	tests := []struct {
		name  string
		items Items
		want  bool
	}{
		{
			name: "returns true when federation target exists",
			items: Items{
				&Item{Targets: []string{"https://example.com/_matrix/federation/v1/version"}},
			},
			want: true,
		},
		{
			name: "returns false when no federation target",
			items: Items{
				&Item{Targets: []string{"https://example.com/some/other/endpoint"}},
			},
			want: false,
		},
		{
			name:  "returns false when no items",
			items: Items{},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.items.ContainsFederation(); got != tt.want {
				t.Errorf("ContainsFederation() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestItems_ContainsDelegation(t *testing.T) {
	tests := []struct {
		name  string
		items Items
		want  bool
	}{
		{
			name: "returns true when delegation target exists",
			items: Items{
				&Item{Targets: []string{"https://example.com/.well-known/matrix/server"}},
			},
			want: true,
		},
		{
			name: "returns false when no delegation target",
			items: Items{
				&Item{Targets: []string{"https://example.com/some/other/endpoint"}},
			},
			want: false,
		},
		{
			name:  "returns false when no items",
			items: Items{},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.items.ContainsDelegation(); got != tt.want {
				t.Errorf("ContainsDelegation() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestItems_ContainsMSC1929(t *testing.T) {
	tests := []struct {
		name  string
		items Items
		want  bool
	}{
		{
			name: "returns true when MSC1929 target exists",
			items: Items{
				&Item{Targets: []string{"https://example.com/.well-known/matrix/support"}},
			},
			want: true,
		},
		{
			name: "returns false when no MSC1929 target",
			items: Items{
				&Item{Targets: []string{"https://example.com/some/other/endpoint"}},
			},
			want: false,
		},
		{
			name:  "returns false when no items",
			items: Items{},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.items.ContainsMSC1929(); got != tt.want {
				t.Errorf("ContainsMSC1929() = %v, want %v", got, tt.want)
			}
		})
	}
}
