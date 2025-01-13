package cmd

import (
	"github.com/manifoldco/promptui"
)

func selectArticleFromPrompt(articles []string) (int, error) {
	templates := &promptui.SelectTemplates{
		Label:    "{{ . }}",
		Active:   "\U0001F336 {{ . | red }}",
		Inactive: "  {{ . | cyan }}",
		Selected: "\U0001F336 {{ . | red | cyan }}",
	}

	prompt := promptui.Select{
		Label:        "Select Article",
		Items:        articles,
		Templates:    templates,
		Size:         4,
		HideSelected: true,
		Stdout:       NoBellStdout,
	}

	i, _, err := prompt.Run()
	return i, err
}
