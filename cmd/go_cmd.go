package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	goPkgPath    string
	goRootPath   string
	goOutputFile string
)

var goCmd = &cobra.Command{
	Use:   "go",
	Short: "Scrape Go package documentation",
	RunE: func(cmd *cobra.Command, args []string) error {
		if goPkgPath == "" {
			return fmt.Errorf("package path is required")
		}
		if goOutputFile == "" {
			goOutputFile = fmt.Sprintf("docs_%d.txt", time.Now().Unix())
		}

		// Determine Root Path if not provided
		if goRootPath == "" {
			parts := strings.Split(goPkgPath, "/")
			if len(parts) >= 3 {
				goRootPath = strings.Join(parts[:3], "/")
				// Check for version major e.g. /v2/
				if len(parts) >= 4 && strings.HasPrefix(parts[3], "v") && isNumeric(parts[3][1:]) {
					goRootPath = goRootPath + "/" + parts[3]
				}
			} else {
				goRootPath = goPkgPath
			}
		}

		tmpDir, err := os.MkdirTemp("", "go-docs-repo")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmpDir)

		gitRoot := goRootPath
		// Handling version suffix removal for git clone url logic from bash script
		// bash: if [[ "$GIT_ROOT" =~ /v[0-9]+$ ]]; then ...
		// But in bash it was checking user input. Go `go get` usually handles this but we are using `git clone`.
		// If gitRoot ends with /v2, we strip it for the clone URL usually?
		// The bash script did:
		// GIT_ROOT="$ROOT_PATH"
		// if [[ "$GIT_ROOT" =~ /v[0-9]+$ ]]; then GIT_ROOT="${GIT_ROOT%/*}"; fi
		// git clone "https://$GIT_ROOT"

		cloneUrl := "https://" + gitRoot
		if idx := strings.LastIndex(gitRoot, "/v"); idx != -1 {
			// Basic check if it looks like a version
			suffix := gitRoot[idx+1:]
			if len(suffix) > 1 && isNumeric(suffix[1:]) {
				cloneUrl = "https://" + gitRoot[:idx]
			}
		}

		fmt.Printf("Cloning %s...\n", cloneUrl)
		gitCmd := exec.Command("git", "clone", cloneUrl, tmpDir, "--quiet")
		gitCmd.Stdout = os.Stdout
		gitCmd.Stderr = os.Stderr
		if err := gitCmd.Run(); err != nil {
			return fmt.Errorf("git clone failed: %w", err)
		}

		// Prepare output file
		outFile, err := os.Create(goOutputFile)
		if err != nil {
			return err
		}
		defer outFile.Close()

		// Cat README
		entries, _ := os.ReadDir(tmpDir)
		for _, e := range entries {
			if strings.HasPrefix(strings.ToLower(e.Name()), "readme") {
				content, _ := os.ReadFile(filepath.Join(tmpDir, e.Name()))
				outFile.Write(content)
				break
			}
		}

		fmt.Fprintln(outFile, "\n\n--- GO DOC OUTPUT ---\n")

		// Calculate relative path for go doc
		// REL_PATH logic from bash is complex.
		// It tries to find the relative path of the pkg inside the repo.
		// REPO_BASE construction matches ROOT_PATH logic roughly.

		var relPath string
		repoBase := goRootPath // Assuming this matches what we cloned or the root of the module
		// Bash script logic:
		parts := strings.Split(goPkgPath, "/")
		if len(parts) >= 3 {
			repoBase = strings.Join(parts[:3], "/")
		} else {
			repoBase = gitRoot
		}

		pathAfterRepo := strings.TrimPrefix(goPkgPath, repoBase)
		// Removing leading slash
		pathAfterRepo = strings.TrimPrefix(pathAfterRepo, "/")

		// Check v2
		segParts := strings.Split(pathAfterRepo, "/")
		if len(segParts) > 0 && strings.HasPrefix(segParts[0], "v") && isNumeric(segParts[0][1:]) {
			pathAfterRepo = strings.TrimPrefix(pathAfterRepo, segParts[0])
			pathAfterRepo = strings.TrimPrefix(pathAfterRepo, "/")
		}

		if pathAfterRepo != "" {
			relPath = "./" + pathAfterRepo
		} else {
			relPath = "."
		}

		fmt.Printf("Running go doc on %s inside %s\n", relPath, tmpDir)

		// Run go doc in the repo dir
		// go doc might require go.mod to be valid?
		// "go doc -all ."

		docCmd := exec.Command("go", "doc", "-all", relPath)
		docCmd.Dir = tmpDir
		docCmd.Stdout = outFile
		// docCmd.Stderr = os.Stderr // Bash ignored stderr and printed "No Go Doc found" if failed, let's allow it to fail silently-ish or capture output

		if err := docCmd.Run(); err != nil {
			fmt.Fprintln(outFile, "No Go Doc found for this package.")
		}

		fmt.Printf("Success! Saved to %s\n", goOutputFile)

		return nil
	},
}

func isNumeric(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func init() {
	rootCmd.AddCommand(goCmd)

	goCmd.Flags().StringVarP(&goPkgPath, "package", "p", "", "Package Path (e.g. github.com/user/repo/sub)")
	goCmd.Flags().StringVarP(&goRootPath, "root", "r", "", "Root Package Path (optional)")
	goCmd.Flags().StringVarP(&goOutputFile, "output", "o", "", "Output filename")
}
