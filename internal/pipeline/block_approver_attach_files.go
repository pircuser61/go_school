package pipeline

import (
	"github.com/iancoleman/orderedmap"
	file_registry "gitlab.services.mts.ru/jocasta/pipeliner/internal/fileregistry"
)

func isNeedAddDownloadImage(description []orderedmap.OrderedMap) bool {
	for _, v := range description {
		links, ok := v.Get("attachLinks")
		if ok {
			attachFiles, ok := links.([]file_registry.AttachInfo)
			if ok && len(attachFiles) != 0 {
				return true
			}
		}
	}

	return false
}
