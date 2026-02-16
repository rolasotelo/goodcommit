package main

import (
	"fmt"
	"log"

	"github.com/charmbracelet/huh"
)

func main() {
	var ready bool
	var commitMessage string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Are you ready to write the best version of your commit? 🥦").
				Value(&ready),
		),
		huh.NewGroup(
			huh.NewText().
				Title("Enter your commit message:").
				Value(&commitMessage).
				Placeholder("Add user authentication feature"),
		).WithHideFunc(func() bool { return !ready }),
	)

	err := form.Run()
	if err != nil {
		log.Fatal(err)
	}

	if ready && commitMessage != "" {
		fmt.Printf("\nYour commit message: %s\n", commitMessage)
	} else {
		fmt.Println("\nNo commit message created.")
	}
}
