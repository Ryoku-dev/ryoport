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
	groupPinned
	groupRunning
	groupBecoming
	groupStopped
)

type resourceGroup struct {
	Kind      groupKind
	Title     string
	Resources []model.Resource
}

var groupTitles = [...]string{"Needs you", "Pinned", "Running", "Becoming", "Stopped"}

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
	var counts [len(groupTitles)]int
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
	for _, resource := range resources {
		kind := groupOf(resource)
		for i := range groups {
			if groups[i].Kind == kind {
				groups[i].Resources = append(groups[i].Resources, resource)
				break
			}
		}
	}
	return groups
}

func groupOf(resource model.Resource) groupKind {
	// A server with a failed unit is still up and answering; it reads as
	// running with an amber word rather than waiting on the person.
	var kind groupKind
	if resource.State == model.StateDegraded {
		kind = groupRunning
	} else {
		switch resource.State.Tone() {
		case model.ToneAttention, model.ToneBad:
			kind = groupNeeds
		case model.ToneAlive:
			kind = groupRunning
		case model.ToneBusy:
			kind = groupBecoming
		default:
			kind = groupStopped
		}
	}
	if resource.Pinned && kind != groupNeeds {
		return groupPinned
	}
	return kind
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
