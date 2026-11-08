package palette

import (
	"reflect"
	"testing"
)

func TestRank(t *testing.T) {
	tests := []struct {
		name  string
		query string
		items []rankItem
		want  []int
	}{
		{
			name:  "prefix before word start before scattered",
			query: "alp",
			items: []rankItem{
				{Text: "A long path"},
				{Text: "Open alpha"},
				{Text: "Alpha server"},
			},
			want: []int{2, 1, 0},
		},
		{
			name:  "exact before longer prefix",
			query: "open",
			items: []rankItem{
				{Text: "Open settings"},
				{Text: "Open"},
			},
			want: []int{1, 0},
		},
		{
			name:  "ties preserve input order",
			query: "ally",
			items: []rankItem{
				{Text: "Connect ally"},
				{Text: "Connect ally"},
				{Text: "Connect ally"},
			},
			want: []int{0, 1, 2},
		},
		{
			name:  "empty query puts resources before actions",
			query: "",
			items: []rankItem{
				{Text: "Refresh"},
				{Text: "ally", Resource: true},
				{Text: "Settings"},
				{Text: "windows-11", Resource: true},
			},
			want: []int{1, 3, 0, 2},
		},
		{
			name:  "unmatched items are omitted",
			query: "gpu",
			items: []rankItem{
				{Text: "ally"},
				{Text: "Open GPU settings"},
			},
			want: []int{1},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := rank(test.query, test.items); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("rank(%q) = %v, want %v", test.query, got, test.want)
			}
		})
	}
}
