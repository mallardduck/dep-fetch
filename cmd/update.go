package cmd

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/go-git/go-billy/v5/osfs"
	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v3"

	"github.com/mallardduck/dep-fetch/internal/config"
	"github.com/mallardduck/dep-fetch/internal/fetch"
	gh "github.com/mallardduck/dep-fetch/internal/github"
	"github.com/mallardduck/dep-fetch/internal/release"
)

var renovateLocalRe = regexp.MustCompile(`(renovate-local:\s*[a-zA-Z0-9_-]+)=.*`)

var updateCmd = &cobra.Command{
	Use:   "update [tool] [version]",
	Short: "Update a tool's version and checksums in the configuration file",
	Long: `Update a tool's version and checksums in the configuration file.
The command first attempts to download the checksum file (using checksum_template if provided).
If the checksum file is missing or incomplete, it falls back to downloading each asset 
individually and calculating its SHA-256 checksum.
If version is "latest", the latest release tag is fetched from GitHub.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		toolName := args[0]
		newVersion := args[1]

		fs := osfs.New(".")
		cfg, _, err := config.Load(fs, configFile, "")
		if err != nil {
			return err
		}

		var targetTool *config.Tool
		for i := range cfg.Tools {
			if cfg.Tools[i].Name == toolName {
				targetTool = &cfg.Tools[i]
				break
			}
		}

		if targetTool == nil {
			return fmt.Errorf("tool %q not found in config", toolName)
		}

		if newVersion == "latest" {
			v, err := gh.LatestRelease(targetTool.Owner(), targetTool.Repo())
			if err != nil {
				return fmt.Errorf("fetching latest release for %s/%s: %w", targetTool.Owner(), targetTool.Repo(), err)
			}
			newVersion = v
		}

		fmt.Printf("Updating %s to %s...\n", toolName, newVersion)

		newChecksums := make(map[string]string)
		if targetTool.Mode == config.ModePinned {
			vars := release.Vars{
				Name:    targetTool.Name,
				Version: newVersion,
			}
			checksumAsset := release.Render(targetTool.ChecksumTemplate(), vars)
			checksumURL := release.AssetURL(targetTool.Owner(), targetTool.Repo(), newVersion, checksumAsset)

			fmt.Printf("  Attempting to use checksum file %s...\n", checksumAsset)
			var checksumBuf bytes.Buffer
			err := gh.DownloadAsset(checksumURL, &checksumBuf)

			checksumsFoundInFile := false
			if err == nil {
				checksumData := checksumBuf.Bytes()
				allFound := true
				tempChecksums := make(map[string]string)
				for plat := range targetTool.Checksums {
					parts := strings.Split(plat, "/")
					if len(parts) != 2 {
						return fmt.Errorf("invalid platform format: %s", plat)
					}
					goos, goarch := parts[0], parts[1]
					v := vars
					v.OS = goos
					v.Arch = goarch
					assetName := release.Render(targetTool.BinaryTemplate(), v)

					sum, err := fetch.ParseChecksumFile(checksumData, assetName)
					if err != nil {
						allFound = false
						fmt.Printf("    %s not found in checksum file\n", assetName)
						break
					}
					tempChecksums[plat] = sum
				}
				if allFound {
					newChecksums = tempChecksums
					checksumsFoundInFile = true
					fmt.Printf("  Found all checksums in %s\n", checksumAsset)
				}
			} else {
				fmt.Printf("  Could not download checksum file %s: %v\n", checksumAsset, err)
			}

			if !checksumsFoundInFile {
				fmt.Printf("  Falling back to downloading individual assets...\n")
				for plat := range targetTool.Checksums {
					parts := strings.Split(plat, "/")
					goos, goarch := parts[0], parts[1]

					v := vars
					v.OS = goos
					v.Arch = goarch
					assetName := release.Render(targetTool.BinaryTemplate(), v)
					assetURL := release.AssetURL(targetTool.Owner(), targetTool.Repo(), newVersion, assetName)

					fmt.Printf("  Fetching %s/%s (%s)...\n", goos, goarch, assetName)
					var buf bytes.Buffer
					if err := gh.DownloadAsset(assetURL, &buf); err != nil {
						return fmt.Errorf("downloading %s: %w", assetName, err)
					}
					newChecksums[plat] = fetch.Sha256Hex(buf.Bytes())
				}
			}
		}

		// Update the YAML file line-by-line to preserve all formatting and comments
		if err := updateYAMLLines(config.ResolveConfigPath(configFile), toolName, newVersion, newChecksums); err != nil {
			return err
		}

		fmt.Printf("Successfully updated %s to %s in %s\n", toolName, newVersion, config.ResolveConfigPath(configFile))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
}

func updateYAMLLines(path, toolName, version string, checksums map[string]string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var node yaml.Node
	if err := yaml.Unmarshal(data, &node); err != nil {
		return err
	}

	if node.Kind != yaml.DocumentNode || len(node.Content) == 0 {
		return fmt.Errorf("invalid YAML document")
	}

	root := node.Content[0]
	var toolsNode *yaml.Node
	for i := 0; i < len(root.Content); i += 2 {
		if root.Content[i].Value == "tools" {
			toolsNode = root.Content[i+1]
			break
		}
	}

	if toolsNode == nil || toolsNode.Kind != yaml.SequenceNode {
		return fmt.Errorf("could not find tools sequence in config")
	}

	var foundToolNode *yaml.Node
	for _, toolNode := range toolsNode.Content {
		for i := 0; i < len(toolNode.Content); i += 2 {
			if toolNode.Content[i].Value == "name" && toolNode.Content[i+1].Value == toolName {
				foundToolNode = toolNode
				break
			}
		}
		if foundToolNode != nil {
			break
		}
	}

	if foundToolNode == nil {
		return fmt.Errorf("tool %q not found in YAML", toolName)
	}

	// We use the exact line indexes to update values as string replacement
	lines := strings.Split(string(data), "\n")

	// Update version
	for i := 0; i < len(foundToolNode.Content); i += 2 {
		if foundToolNode.Content[i].Value == "version" {
			valNode := foundToolNode.Content[i+1]
			lineIdx := valNode.Line - 1

			oldVal := valNode.Value
			searchVal := oldVal
			if valNode.Style == yaml.DoubleQuotedStyle {
				searchVal = `"` + oldVal + `"`
			} else if valNode.Style == yaml.SingleQuotedStyle {
				searchVal = `'` + oldVal + `'`
			}

			newValStr := version
			if valNode.Style == yaml.DoubleQuotedStyle {
				newValStr = `"` + version + `"`
			} else if valNode.Style == yaml.SingleQuotedStyle {
				newValStr = `'` + version + `'`
			}

			lines[lineIdx] = strings.Replace(lines[lineIdx], searchVal, newValStr, 1)
			break
		}
	}

	// Update checksums
	if len(checksums) > 0 {
		for i := 0; i < len(foundToolNode.Content); i += 2 {
			if foundToolNode.Content[i].Value == "checksums" {
				checksumsNode := foundToolNode.Content[i+1]
				for j := 0; j < len(checksumsNode.Content); j += 2 {
					platNode := checksumsNode.Content[j]
					valNode := checksumsNode.Content[j+1]
					plat := platNode.Value

					if sum, ok := checksums[plat]; ok {
						lineIdx := valNode.Line - 1

						oldVal := valNode.Value
						searchVal := oldVal
						if valNode.Style == yaml.DoubleQuotedStyle {
							searchVal = `"` + oldVal + `"`
						} else if valNode.Style == yaml.SingleQuotedStyle {
							searchVal = `'` + oldVal + `'`
						}

						newValStr := sum
						if valNode.Style == yaml.DoubleQuotedStyle {
							newValStr = `"` + sum + `"`
						} else if valNode.Style == yaml.SingleQuotedStyle {
							newValStr = `'` + sum + `'`
						}

						lines[lineIdx] = strings.Replace(lines[lineIdx], searchVal, newValStr, 1)
						lines[lineIdx] = renovateLocalRe.ReplaceAllString(lines[lineIdx], "${1}="+version)
					}
				}
				break
			}
		}
	}

	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644)
}
