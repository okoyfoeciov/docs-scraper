package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

var (
	ghRepoURL   string
	ghPaths     []string
	ghExts      []string
	ghOutputFile string
)

var githubCmd = &cobra.Command{
	Use:   "github",
	Short: "Scrape markdown from a GitHub repository",
	RunE: func(cmd *cobra.Command, args []string) error {
		if ghRepoURL == "" {
			return fmt.Errorf("repository URL is required")
		}
		if len(ghExts) == 0 {
			ghExts = []string{"mdx", "md"}
		}
		if ghOutputFile == "" {
			ghOutputFile = "output.md"
		}
		
		// If ghPaths is empty, default to root "."? No, the bash script says:
		// if [ ${#docs_dirs[@]} -eq 0 ]; then docs_dirs=(".")
		if len(ghPaths) == 0 {
			ghPaths = []string{"."}
		}

		tmpDir, err := os.MkdirTemp("", "gh-docs")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmpDir)

		fmt.Printf("Cloning %s...\n", ghRepoURL)
		gitCmd := exec.Command("git", "clone", "--depth", "1", "--recurse-submodules", "--shallow-submodules", ghRepoURL, tmpDir)
		gitCmd.Stdout = os.Stdout
		gitCmd.Stderr = os.Stderr
		if err := gitCmd.Run(); err != nil {
			// Try without specific clone options if it fails? No, keep it simple.
			return fmt.Errorf("git clone failed: %w", err)
		}

		outFile, err := os.Create(ghOutputFile)
		if err != nil {
			return err
		}
		defer outFile.Close()

		// Regex for cleaning imports/exports
		// Match lines starting with optional whitespace, then import ..., ending with ; 
		// Match lines starting with optional whitespace, then export ...
		importRe := regexp.MustCompile(`^\s*import\s.*;\s*$`)
		exportRe := regexp.MustCompile(`^\s*export\s.*$`)

		for _, relSrc := range ghPaths {
			srcPath := filepath.Join(tmpDir, relSrc)
			// Check existence
			if _, err := os.Stat(srcPath); os.IsNotExist(err) {
				fmt.Printf("Warning: Path not found: %s\n", relSrc)
				continue
			}

			err = filepath.Walk(srcPath, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if info.IsDir() {
					return nil
				}
				
				// Check extension
				matchExt := false
				for _, ext := range ghExts {
					ext = strings.TrimPrefix(ext, ".") // ensure clean ext
					if strings.HasSuffix(strings.ToLower(info.Name()), "."+strings.ToLower(ext)) {
						matchExt = true
						break
					}
				}
				if !matchExt {
					return nil
				}

				// Calculate relative path for header
				relFile, _ := filepath.Rel(tmpDir, path)

				fmt.Fprintf(outFile, "<!-- FILE: %s -->\n\n### %s\n\n", relFile, relFile)

				// Process file content line by line
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
				fmt.Printf("Error walking %s: %v\n", srcPath, err)
			}
		}

		fmt.Printf("Done! Saved to %s\n", ghOutputFile)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(githubCmd)

	githubCmd.Flags().StringVar(&ghRepoURL, "url", "", "Git Repository URL")
	githubCmd.Flags().StringSliceVar(&ghPaths, "paths", []string{}, "Paths to include (comma-separated)")
	githubCmd.Flags().StringSliceVar(&ghExts, "exts", []string{"mdx", "md"}, "File extensions to include")
	githubCmd.Flags().StringVarP(&ghOutputFile, "output", "o", "output.md", "Output file")
}
