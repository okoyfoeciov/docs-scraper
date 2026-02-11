package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

var (
	localPaths      []string
	localExts       []string
	localOutputFile string
)

var localCmd = &cobra.Command{
	Use:   "local",
	Short: "Scrape markdown from local directories",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(localPaths) == 0 {
			return fmt.Errorf("at least one source path is required")
		}
		if len(localExts) == 0 {
			localExts = []string{"mdx", "md"}
		}
		if localOutputFile == "" {
			localOutputFile = "output.md"
		}

		outFile, err := os.Create(localOutputFile)
		if err != nil {
			return err
		}
		defer outFile.Close()

		importRe := regexp.MustCompile(`^\s*import\s.*;\s*$`)
		exportRe := regexp.MustCompile(`^\s*export\s.*$`)

		for _, rootPath := range localPaths {
			// Resolve absolute path
			absRoot, err := filepath.Abs(rootPath)
			if err != nil {
				fmt.Printf("Warning: Cannot resolve path %s: %v\n", rootPath, err)
				continue
			}

			if _, err := os.Stat(absRoot); os.IsNotExist(err) {
				fmt.Printf("Warning: Path not found: %s\n", absRoot)
				continue
			}

			err = filepath.Walk(absRoot, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if info.IsDir() {
					return nil
				}

				// Check extension
				matchExt := false
				for _, ext := range localExts {
					ext = strings.TrimPrefix(ext, ".")
					if strings.HasSuffix(strings.ToLower(info.Name()), "."+strings.ToLower(ext)) {
						matchExt = true
						break
					}
				}
				if !matchExt {
					return nil
				}

				// Check if inside root (symlinks might lead outside, though Walk follows structure, but user script used realpath checks)
				// filepath.Walk does not follow symlinks by default, but the bash script used `find -L`.
				// To truly support `find -L`, we'd need to explicitly follow symlinks.
				// For simplicity in this v1, typical usages are direct files.
				// However, let's just process.

				relFile, _ := filepath.Rel(absRoot, path)

				fmt.Fprintf(outFile, "<!-- FILE: %s -->\n\n### %s\n\n", relFile, relFile)

				f, err := os.Open(path)
				if err != nil {
					return err
				}
				defer f.Close()

				scanner := bufio.NewScanner(f)
				for scanner.Scan() {
					line := scanner.Text()
					if importRe.MatchString(line) {
						continue
					}
					if exportRe.MatchString(line) {
						continue
					}
					fmt.Fprintln(outFile, line)
				}

				fmt.Fprintln(outFile, "\n\n---\n\n")

				return nil
			})
			if err != nil {
				fmt.Printf("Error walking %s: %v\n", absRoot, err)
			}
		}

		fmt.Printf("Done! Saved to %s\n", localOutputFile)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(localCmd)

	localCmd.Flags().StringSliceVar(&localPaths, "paths", []string{}, "Local source folder(s) (comma-separated)")
	localCmd.Flags().StringSliceVar(&localExts, "exts", []string{"mdx", "md"}, "File extensions to include")
	localCmd.Flags().StringVarP(&localOutputFile, "output", "o", "output.md", "Output file")
}
