package utils

import (
	"fmt"
	"regexp"
	"strings"
)

func MakeTaskTitle(versionTitle, customTitle string, isTest bool) (res string) {
	res = versionTitle

	if customTitle != "" {
		res = customTitle
	}

	if isTest {
		res = fmt.Sprintf("%s (ТЕСТОВАЯ ЗАЯВКА)", res)
	}

	return res
}

func GetAttachmentsIds(text string) []string {
	res := make([]string, 0)
	uuidPattern := "[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}"

	text = strings.ReplaceAll(text, "\"", "")
	text = strings.ReplaceAll(text, "\\", "")

	patterns := fmt.Sprintf("%s|%s|%s",
		"file_id:"+uuidPattern,
		"external_link:"+uuidPattern,
		"attachment:"+uuidPattern,
	)

	r := regexp.MustCompile(patterns)
	matchedSubstrings := r.FindAllString(text, -1)

	if matchedSubstrings != nil {
		r = regexp.MustCompile(uuidPattern)
		for i := range matchedSubstrings {
			fileIds := r.FindAllString(matchedSubstrings[i], -1)
			if fileIds != nil {
				res = append(res, fileIds...)
			}
		}
	}

	return res
}

func CleanUnexpectedSymbols(s string) string {
	replacements := map[string]string{
		"\\t":  "",
		"\t":   "",
		"\\n":  "",
		"\n":   "",
		"\r":   "",
		"\\r":  "",
		"\"\"": "",
		"\"":   "''",
	}

	for old, news := range replacements {
		s = strings.ReplaceAll(s, old, news)
	}

	return strings.ReplaceAll(s, "\\", "")
}
