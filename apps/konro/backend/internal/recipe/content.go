package recipe

import "strings"

type Content struct {
	Title       string   `json:"title"`
	Servings    string   `json:"servings"`
	Ingredients []string `json:"ingredients"`
	Steps       []string `json:"steps"`
}

func ValidateContent(c Content) error {
	if len(c.Title) > 300 || len(c.Servings) > 200 || strings.ContainsAny(c.Title+c.Servings, "\r\n\x00") || len(c.Ingredients) > 100 || len(c.Steps) > 150 {
		return ErrInvalid
	}
	for _, list := range [][]string{c.Ingredients, c.Steps} {
		for _, line := range list {
			if strings.TrimSpace(line) == "" || len(line) > 3000 || strings.ContainsAny(line, "\r\n\x00") {
				return ErrInvalid
			}
		}
	}
	return nil
}
