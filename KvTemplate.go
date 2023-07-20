package main

import (
	"regexp"
	"strings"
)

type KvTemplate struct {
	template string
}

func NewKvTemplate(template string) *KvTemplate {
	return &KvTemplate{template: template}
}

func (t *KvTemplate) Render(values map[string]string) string {
	re := regexp.MustCompile(`\${(.*?)}`)
	result := re.ReplaceAllStringFunc(
		t.template, func(match string) string {
			key := strings.TrimSuffix(strings.TrimPrefix(match, "${"), "}")
			return values[key]
		},
	)

	return result
}
