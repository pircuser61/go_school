package nocache

import (
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

//nolint:gochecknoglobals // в данном проекте было бы проще отключить этот линтер
var (
	englishCheck = regexp.MustCompile(`[^а-яё]`)
	newStringRm  = regexp.MustCompile(`(\s\s+)|(\n)`)

	usernameEqFilter            = fmt.Sprintf("(username eq %q)", usernamePH)
	usernameEqOnlyEnabledFilter = fmt.Sprintf("((username eq %q) and (enabled eq true))", usernamePH)

	usernameFilter = fmt.Sprintf("(username sw %q)", usernamePH)
	onePartFilter  = fmt.Sprintf("(lastName sw %q)", name1PH)
	twoPartFilter  = fmt.Sprintf(`(((firstName eq "%s") and (lastName sw "%s")) or
											((firstName sw "%s") and (lastName eq "%s"))
)`,
		name1PH, name2PH,
		name2PH, name1PH)
	threePartFilter = fmt.Sprintf(`(((lastName eq "%s") and (firstName eq "%s") and (attributes.fullname co "%s")) or 
											((lastName eq "%s") and (firstName eq "%s") and (attributes.fullname co "%s")) or 
											((lastName eq "%s") and (firstName sw "%s") and (attributes.fullname co "%s")) or 
											((lastName eq "%s") and (firstName sw "%s") and (attributes.fullname co "%s")) or 
											((lastName sw "%s") and (firstName eq "%s") and (attributes.fullname co "%s")) or 
											((lastName sw "%s") and (firstName eq "%s") and (attributes.fullname co "%s"))  
)`,
		name1PH, name2PH, name3PH,
		name2PH, name1PH, name3PH,
		name1PH, name3PH, name2PH,
		name2PH, name3PH, name1PH,
		name3PH, name1PH, name2PH,
		name3PH, name2PH, name1PH,
	)
)

// nolint:staticcheck // Cant use cases.Title
func defineFilter(input string, oneWord bool, filter []string) string {
	parts := strings.Split(strings.ToLower(input), " ")
	caser := cases.Title(language.Tag{})

	var q string

	if len(parts) == 1 || oneWord {
		if englishCheck.FindString(strings.ToLower(parts[0])) != "" {
			q = strings.Replace(usernameFilter, usernamePH, parts[0], 1)
		} else {
			q = strings.Replace(onePartFilter, name1PH, caser.String(parts[0]), 1)
		}
	}

	if len(parts) == 2 {
		q = strings.ReplaceAll(twoPartFilter, name1PH, caser.String(parts[0]))
		q = strings.ReplaceAll(q, name2PH, caser.String(parts[1]))
	}

	if len(parts) > 2 {
		q = strings.ReplaceAll(threePartFilter, name1PH, caser.String(parts[0]))
		q = strings.ReplaceAll(q, name2PH, caser.String(parts[1]))
		q = strings.ReplaceAll(q, name3PH, caser.String(parts[2]))
	}

	if len(filter) > 0 {
		companyOptions := make([]string, 0, len(filter))

		for _, f := range filter {
			companyOptions = append(companyOptions, fmt.Sprintf(companyFilterOption, f))
		}

		q += fmt.Sprintf(" and (%s)", strings.Join(companyOptions, " or "))
	}

	return newStringRm.ReplaceAllString(q, " ")
}
