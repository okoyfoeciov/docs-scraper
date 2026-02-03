package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "docs-scraper",
	Short: "A tool to scrape documentation from various sources",
	Long: `Docs Scraper is a CLI tool that helps you download and consolidate documentation 
from Rust crates, Go packages, GitHub repositories, and local directories 
into a single Markdown file.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
