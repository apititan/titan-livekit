package handlers

import "github.com/microcosm-cc/bluemonday"

func CreateSanitizer() *bluemonday.Policy {
	policy := bluemonday.UGCPolicy()
	policy.AllowAttrs("style").OnElements("span", "p", "strong", "em", "s", "u", "img", "mark")
	policy.AllowAttrs("class").OnElements("img")
	policy.AllowAttrs("target").OnElements("a")
	return policy
}
