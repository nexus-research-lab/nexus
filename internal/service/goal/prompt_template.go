package goal

import "strings"

func renderGoalPromptTemplate(template string, values map[string]string) string {
	replacements := make([]string, 0, len(values)*2)
	for key, value := range values {
		replacements = append(replacements, "{{ "+key+" }}", value)
	}
	return strings.NewReplacer(replacements...).Replace(template)
}
