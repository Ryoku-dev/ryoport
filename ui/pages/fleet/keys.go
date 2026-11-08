package fleet

import "github.com/ryoku-dev/ryoport/internal/model"

func moveSelection(resources []model.Resource, current model.ID, delta int) model.ID {
	if len(resources) == 0 {
		return ""
	}

	index := -1
	for i := range resources {
		if resources[i].ID == current {
			index = i
			break
		}
	}
	if index < 0 {
		if delta < 0 {
			return resources[len(resources)-1].ID
		}
		return resources[0].ID
	}

	index += delta
	if index < 0 {
		index = 0
	}
	if index >= len(resources) {
		index = len(resources) - 1
	}
	return resources[index].ID
}

func resourceByID(resources []model.Resource, id model.ID) (model.Resource, bool) {
	for _, resource := range resources {
		if resource.ID == id {
			return resource, true
		}
	}
	return model.Resource{}, false
}
