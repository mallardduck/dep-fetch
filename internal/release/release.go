package release

import (
	"fmt"
	"regexp"
	"strings"
)

// Vars holds the substitution values for release asset name patterns declared
// in .bin-deps.yaml (binary_template, checksum_template).
type Vars struct {
	Name    string
	OS      string
	Arch    string
	ArchAlt string // alternative arch name, e.g. x86_64 for amd64
	Version string // e.g. v0.18.0
}

// tokenRe matches {variable} and {variable|modifier1,modifier2,...} template tokens.
var tokenRe = regexp.MustCompile(`\{([^}|]+)(?:\|([^}]*))?\}`)

// Render substitutes all template variables in a release asset name pattern.
// Tokens take the form {variable} or {variable|modifier1,modifier2,...}.
// Modifiers are applied left-to-right.
//
// Supported modifiers:
//   - upper            — strings.ToUpper
//   - lower            — strings.ToLower
//   - title            — capitalise first character only (e.g. darwin → Darwin)
//   - trimprefix:ARG   — strings.TrimPrefix(val, ARG)
//   - trimsuffix:ARG   — strings.TrimSuffix(val, ARG)
//
// Design restriction: modifier arguments (the part after ':') must not contain a
// comma, because commas are the modifier separator. Any future modifier whose
// argument requires a comma will need its own delimiter or a revised parsing
// strategy.
//
// Unknown variables or modifiers are left as-is.
func Render(pattern string, v Vars) string {
	vars := map[string]string{
		"name":     v.Name,
		"os":       v.OS,
		"arch":     v.Arch,
		"arch_alt": v.ArchAlt,
		"version":  v.Version,
	}
	return tokenRe.ReplaceAllStringFunc(pattern, func(token string) string {
		m := tokenRe.FindStringSubmatch(token)
		val, ok := vars[m[1]]
		if !ok {
			return token
		}
		if m[2] == "" {
			return val
		}
		for mod := range strings.SplitSeq(m[2], ",") {
			name, arg, _ := strings.Cut(mod, ":")
			switch name {
			case "upper":
				val = strings.ToUpper(val)
			case "lower":
				val = strings.ToLower(val)
			case "title":
				if val != "" {
					val = strings.ToUpper(val[:1]) + val[1:]
				}
			case "trimprefix":
				val = strings.TrimPrefix(val, arg)
			case "trimsuffix":
				val = strings.TrimSuffix(val, arg)
			}
		}
		return val
	})
}

// AssetURL returns the download URL for a named asset in a GitHub release.
func AssetURL(owner, repo, tag, assetName string) string {
	return fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/%s", owner, repo, tag, assetName)
}
