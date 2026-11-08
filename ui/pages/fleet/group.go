package fleet

import (
	"strings"
	"unicode"

	"github.com/ryoku-dev/ryoport/internal/model"
	"github.com/ryoku-dev/ryoport/ui/env"
)

type groupKind uint8

const (
	groupNeeds groupKind = iota
	groupRunning
	groupBecoming
	groupStopped
)

type resourceGroup struct {
	Kind      groupKind
	Title     string
	Resources []model.Resource
}

var groupTitles = [...]string{"Needs you", "Running", "Becoming", "Stopped"}

func filterResources(resources []model.Resource, filter env.Filter, query string) []model.Resource {
	needle := foldedRunes(strings.TrimSpace(query))
	filtered := make([]model.Resource, 0, len(resources))
	for _, resource := range resources {
		if filter.Match(resource.Kind) && queryMatches(resource, needle) {
			filtered = append(filtered, resource)
		}
	}
	return filtered
}

func queryMatches(resource model.Resource, needle []rune) bool {
	if len(needle) == 0 {
		return true
	}
	if containsFold(resource.Name, needle) || containsFold(resource.Detail, needle) {
		return true
	}
	for _, tag := range resource.Tags {
		if containsFold(tag, needle) {
			return true
		}
	}
	return false
}

func foldedRunes(s string) []rune {
	runes := []rune(s)
	for i, r := range runes {
		runes[i] = unicode.ToLower(r)
	}
	return runes
}

func containsFold(value string, needle []rune) bool {
	if len(needle) == 0 {
		return true
	}
	for start := range value {
		matched := 0
		for _, r := range value[start:] {
			if unicode.ToLower(r) != needle[matched] {
				break
			}
			matched++
			if matched == len(needle) {
				return true
			}
		}
	}
	return false
}

func groupResources(resources []model.Resource) []resourceGroup {
	var counts [4]int
	for _, resource := range resources {
		counts[groupOf(resource)]++
	}

	groups := make([]resourceGroup, 0, len(counts))
	for kind, count := range counts {
		if count == 0 {
			continue
		}
		groups = append(groups, resourceGroup{
			Kind:      groupKind(kind),
			Title:     groupTitles[kind],
			Resources: make([]model.Resource, 0, count),
		})
	}
	for pinned := true; ; pinned = false {
		for _, resource := range resources {
			if resource.Pinned != pinned {
				continue
			}
			kind := groupOf(resource)
			for i := range groups {
				if groups[i].Kind == kind {
					groups[i].Resources = append(groups[i].Resources, resource)
					break
				}
			}
		}
		if !pinned {
			break
		}
	}
	return groups
}

func groupOf(resource model.Resource) groupKind {
	switch resource.State.Tone() {
	case model.ToneAttention, model.ToneBad:
		return groupNeeds
	case model.ToneAlive:
		return groupRunning
	case model.ToneBusy:
		return groupBecoming
	default:
		return groupStopped
	}
}

func fleetCounts(resources []model.Resource) [4]int {
	counts := [4]int{len(resources)}
	for _, resource := range resources {
		switch resource.Kind {
		case model.KindMachine, model.KindPassthrough:
			counts[1]++
		case model.KindServer:
			counts[2]++
		case model.KindConsole:
			counts[3]++
		}
	}
	return counts
}
