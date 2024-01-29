package pipeline

import (
	"github.com/iancoleman/orderedmap"
	file_registry "gitlab.services.mts.ru/jocasta/pipeliner/internal/fileregistry"
)

func (gb *GoExecutionBlock) downloadImgFromDescription(description []orderedmap.OrderedMap) bool {
	for _, v := range description {
		links, link := v.Get("attachLinks")
		if link {
			attachFiles, ok := links.([]file_registry.AttachInfo)
			if ok && len(attachFiles) != 0 {
				return true
			}
		}
	}

	return false
}
