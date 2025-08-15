package filtering

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type FileFilterEngine struct {
	rootDir         string
	includePatterns []Pattern
	excludePatterns []Pattern
	hasFilesField   bool
	builtinExcludes []Pattern
	builtinIncludes []Pattern
}

type Pattern struct {
	Pattern   string
	IsNegated bool
	IsDir     bool
	Regex     *regexp.Regexp
}

type FilteredFile struct {
	RelativePath string
	AbsolutePath string
	IsDir        bool
	Size         int64
}

type FilterResult struct {
	Files      []FilteredFile
	TotalSize  int64
	FileCount  int
	Excluded   []string
	IncludedBy string // "files", "gpmignore", "npmignore", "gitignore", or "builtin"
}

var builtinAlwaysInclude = []string{
	"package.json",
	"README*",
	"LICENSE*",
	"LICENCE*",
	"CHANGELOG*",
	"HISTORY*",
}

var builtinAlwaysExclude = []string{
	"node_modules/",
	".git/",
	".svn/",
	".hg/",
	"CVS/",
	".DS_Store",
	"*.tgz",
	"*.tar.gz",
	"npm-debug.log*",
	"yarn-debug.log*",
	"yarn-error.log*",
	".npm",
	".nyc_output",
	"coverage/",
	".gpmignore",
	".npmignore",
	".gitignore",
}

func NewFileFilterEngine(rootDir string) (*FileFilterEngine, error) {
	engine := &FileFilterEngine{
		rootDir: rootDir,
	}

	if err := engine.loadBuiltinPatterns(); err != nil {
		return nil, fmt.Errorf("failed to load builtin patterns: %w", err)
	}

	if err := engine.loadFilesField(); err != nil {
		return nil, fmt.Errorf("failed to load files field: %w", err)
	}

	if !engine.hasFilesField {
		if err := engine.loadIgnoreFiles(); err != nil {
			return nil, fmt.Errorf("failed to load ignore files: %w", err)
		}
	}

	return engine, nil
}

func (e *FileFilterEngine) loadBuiltinPatterns() error {
	for _, pattern := range builtinAlwaysInclude {
		compiled, err := compilePattern(pattern, false)
		if err != nil {
			return fmt.Errorf("failed to compile builtin include pattern %s: %w", pattern, err)
		}
		e.builtinIncludes = append(e.builtinIncludes, compiled)
	}

	for _, pattern := range builtinAlwaysExclude {
		compiled, err := compilePattern(pattern, false)
		if err != nil {
			return fmt.Errorf("failed to compile builtin exclude pattern %s: %w", pattern, err)
		}
		e.builtinExcludes = append(e.builtinExcludes, compiled)
	}

	return nil
}

func (e *FileFilterEngine) loadFilesField() error {
	packageJSONPath := filepath.Join(e.rootDir, "package.json")
	data, err := os.ReadFile(packageJSONPath)
	if err != nil {
		return fmt.Errorf("failed to read package.json: %w", err)
	}

	var pkg struct {
		Files []string `json:"files"`
	}

	if err := json.Unmarshal(data, &pkg); err != nil {
		return fmt.Errorf("failed to parse package.json: %w", err)
	}

	if len(pkg.Files) > 0 {
		e.hasFilesField = true
		for _, filePattern := range pkg.Files {
			compiled, err := compilePattern(filePattern, false)
			if err != nil {
				return fmt.Errorf("failed to compile files pattern %s: %w", filePattern, err)
			}
			e.includePatterns = append(e.includePatterns, compiled)
		}
	}

	return nil
}

func (e *FileFilterEngine) loadIgnoreFiles() error {
	gpmignorePath := filepath.Join(e.rootDir, ".gpmignore")
	npmignorePath := filepath.Join(e.rootDir, ".npmignore")
	gitignorePath := filepath.Join(e.rootDir, ".gitignore")

	// Priority: .gpmignore > .npmignore > .gitignore
	if _, err := os.Stat(gpmignorePath); err == nil {
		return e.loadIgnoreFile(gpmignorePath)
	}

	if _, err := os.Stat(npmignorePath); err == nil {
		return e.loadIgnoreFile(npmignorePath)
	}

	if _, err := os.Stat(gitignorePath); err == nil {
		return e.loadIgnoreFile(gitignorePath)
	}

	return nil
}

func (e *FileFilterEngine) loadIgnoreFile(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", filename, err)
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		isNegated := strings.HasPrefix(line, "!")
		if isNegated {
			line = line[1:]
		}

		compiled, err := compilePattern(line, isNegated)
		if err != nil {
			continue
		}

		if isNegated {
			e.includePatterns = append(e.includePatterns, compiled)
		} else {
			e.excludePatterns = append(e.excludePatterns, compiled)
		}
	}

	return scanner.Err()
}

func compilePattern(pattern string, isNegated bool) (Pattern, error) {
	p := Pattern{
		Pattern:   pattern,
		IsNegated: isNegated,
		IsDir:     strings.HasSuffix(pattern, "/"),
	}

	regexPattern := patternToRegex(pattern)

	regex, err := regexp.Compile(regexPattern)
	if err != nil {
		return p, fmt.Errorf("failed to compile pattern %s to regex %s: %w", pattern, regexPattern, err)
	}

	p.Regex = regex
	return p, nil
}

func patternToRegex(pattern string) string {
	pattern = regexp.QuoteMeta(pattern)

	pattern = strings.ReplaceAll(pattern, `\*\*`, "DOUBLESTAR")
	pattern = strings.ReplaceAll(pattern, `\*`, "[^/]*")
	pattern = strings.ReplaceAll(pattern, "DOUBLESTAR", ".*")
	pattern = strings.ReplaceAll(pattern, `\?`, "[^/]")

	// Handle root-level patterns (like *.js) vs directory patterns (like src/)
	if strings.HasSuffix(pattern, "/") {
		// Directory pattern - match from root and include all files within
		pattern = strings.TrimPrefix(pattern, "/")
		// Remove trailing slash and match everything that starts with this prefix
		pattern = strings.TrimSuffix(pattern, "/")
		pattern = "^" + pattern + "($|/.*)"
	} else if strings.Contains(pattern, "/") {
		// Path pattern - match from root
		pattern = strings.TrimPrefix(pattern, "/")
		pattern = "^" + pattern
	} else {
		// Root-level file pattern (like *.js) - only match in root directory
		pattern = "^" + pattern + "$"
	}

	return pattern
}

func (e *FileFilterEngine) FilterFiles() (*FilterResult, error) {
	result := &FilterResult{
		Files:    []FilteredFile{},
		Excluded: []string{},
	}

	err := filepath.Walk(e.rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(e.rootDir, path)
		if err != nil {
			return err
		}

		if relPath == "." {
			return nil
		}

		if info.Mode()&os.ModeSymlink != 0 {
			result.Excluded = append(result.Excluded, relPath+" (symlink)")
			return nil
		}

		normalizedPath := filepath.ToSlash(relPath)

		shouldInclude, reason := e.shouldInclude(normalizedPath, info.IsDir())

		if shouldInclude {
			filteredFile := FilteredFile{
				RelativePath: relPath,
				AbsolutePath: path,
				IsDir:        info.IsDir(),
			}

			if !info.IsDir() {
				filteredFile.Size = info.Size()
				result.TotalSize += info.Size()
				result.FileCount++
			}

			result.Files = append(result.Files, filteredFile)

			if result.IncludedBy == "" {
				result.IncludedBy = reason
			}
		} else {
			result.Excluded = append(result.Excluded, relPath)
		}

		return nil
	})

	return result, err
}

// HasFilesField returns whether the engine has a files field configured
func (e *FileFilterEngine) HasFilesField() bool {
	return e.hasFilesField
}

// GetIncludePatterns returns the include patterns for debugging
func (e *FileFilterEngine) GetIncludePatterns() []Pattern {
	return e.includePatterns
}

func (e *FileFilterEngine) shouldInclude(normalizedPath string, isDir bool) (bool, string) {
	// If files field is present, it takes precedence over everything else
	if e.hasFilesField {
		matches := e.matchesFilesField(normalizedPath, isDir)
		if matches {
			return true, "files"
		}
		return false, "files"
	}

	// Builtin includes (always included regardless of other rules)
	if e.matchesBuiltinInclude(normalizedPath) {
		return true, "builtin"
	}

	// Builtin excludes (always excluded regardless of other rules)
	if e.matchesBuiltinExclude(normalizedPath, isDir) {
		return false, "builtin"
	}

	// Check ignore patterns
	if e.matchesExcludePattern(normalizedPath, isDir) {
		if e.matchesIncludePattern(normalizedPath, isDir) {
			return true, "gpmignore/npmignore/gitignore"
		}
		return false, "gpmignore/npmignore/gitignore"
	}

	return true, "default"
}

func (e *FileFilterEngine) matchesBuiltinInclude(normalizedPath string) bool {
	for _, pattern := range e.builtinIncludes {
		if pattern.Regex.MatchString(normalizedPath) {
			return true
		}
	}
	return false
}

func (e *FileFilterEngine) matchesBuiltinExclude(normalizedPath string, isDir bool) bool {
	for _, pattern := range e.builtinExcludes {
		if pattern.IsDir && !isDir {
			continue
		}
		if pattern.Regex.MatchString(normalizedPath) {
			return true
		}
	}
	return false
}

func (e *FileFilterEngine) matchesFilesField(normalizedPath string, isDir bool) bool {
	for _, pattern := range e.includePatterns {
		// Directory patterns should match both directories and files within them
		// File patterns should only match files
		if !pattern.IsDir && isDir {
			continue
		}
		matches := pattern.Regex.MatchString(normalizedPath)
		if matches {
			return true
		}
	}
	return false
}

func (e *FileFilterEngine) matchesExcludePattern(normalizedPath string, isDir bool) bool {
	for _, pattern := range e.excludePatterns {
		// Directory patterns should match both directories and files within them
		// File patterns should only match files (not directories)
		if !pattern.IsDir && isDir {
			continue
		}
		if pattern.Regex.MatchString(normalizedPath) {
			return true
		}
	}
	return false
}

func (e *FileFilterEngine) matchesIncludePattern(normalizedPath string, isDir bool) bool {
	for _, pattern := range e.includePatterns {
		if pattern.IsDir && !isDir {
			continue
		}
		if pattern.Regex.MatchString(normalizedPath) {
			return true
		}
	}
	return false
}
