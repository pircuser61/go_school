package utils

import "fmt"

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
