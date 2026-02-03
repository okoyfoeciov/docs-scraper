package cmd

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

var (
	rustCrateName     string
	rustCrateVersion  string
	rustOutputFile    string
	converterPath     string
)

var rustCmd = &cobra.Command{
	Use:   "rust",
	Short: "Scrape Rust crate documentation",
	RunE: func(cmd *cobra.Command, args []string) error {
		if rustCrateName == "" {
			return fmt.Errorf("crate name is required")
		}
		if rustOutputFile == "" {
			rustOutputFile = fmt.Sprintf("%s.md", rustCrateName)
		}

		// Check converter
		if _, err := os.Stat(converterPath); os.IsNotExist(err) {
			return fmt.Errorf("converter binary not found at %s", converterPath)
		}

		// Download JSON
		url := fmt.Sprintf("https://docs.rs/crate/%s/%s/json.gz", rustCrateName, rustCrateVersion)
		fmt.Printf("Downloading %s...\n", url)
		
		resp, err := http.Get(url)
		if err != nil {
			return fmt.Errorf("failed to download: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("bad status: %s", resp.Status)
		}

		// Create temp file for JSON
		tmpJson, err := os.CreateTemp("", "crate-*.json")
		if err != nil {
			return err
		}
		defer os.Remove(tmpJson.Name())
		defer tmpJson.Close()

		gzReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer gzReader.Close()

		if _, err := io.Copy(tmpJson, gzReader); err != nil {
			return fmt.Errorf("failed to decompress: %w", err)
		}
		tmpJson.Close() // Close specifically before using in command

		// Temp dir for output
		tmpDir, err := os.MkdirTemp("", "crate-docs")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmpDir)

		// Run converter
		fmt.Println("Converting...")
		convertCmd := exec.Command(converterPath, "--json", tmpJson.Name(), "--output", tmpDir)
		convertCmd.Stdout = os.Stdout
		convertCmd.Stderr = os.Stderr
		if err := convertCmd.Run(); err != nil {
			return fmt.Errorf("converter failed: %w", err)
		}

		// Merge files
		outFile, err := os.Create(rustOutputFile)
		if err != nil {
			return err
		}
		defer outFile.Close()

		var mdFiles []string
		err = filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && strings.HasSuffix(info.Name(), ".md") {
				mdFiles = append(mdFiles, path)
			}
			return nil
		})
		if err != nil {
			return err
		}
		sort.Strings(mdFiles)

		for _, file := range mdFiles {
			relPath, _ := filepath.Rel(tmpDir, file)
			
			fmt.Fprintf(outFile, "<!-- FILE: %s -->\n\n", relPath)
			
			content, err := os.ReadFile(file)
			if err != nil {
				return err
			}
			outFile.Write(content)
			outFile.WriteString("\n\n\n")
		}

		fmt.Printf("Success! Saved to %s\n", rustOutputFile)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(rustCmd)

	rustCmd.Flags().StringVarP(&rustCrateName, "crate", "c", "", "Crate name")
	rustCmd.Flags().StringVarP(&rustCrateVersion, "version", "v", "latest", "Crate version")
	rustCmd.Flags().StringVarP(&rustOutputFile, "output", "o", "", "Output filename")
	rustCmd.Flags().StringVar(&converterPath, "converter", "./cargo-doc-md/target/release/cargo-doc-md", "Path to cargo-doc-md binary")
}
