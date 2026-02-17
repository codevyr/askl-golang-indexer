package index

import (
	"net/http"
	"path/filepath"
	"strings"
)

var extensionFiletypes = map[string]string{
	".bash":  "text/x-shellscript",
	".css":   "text/css",
	".go":    "text/x-go",
	".html":  "text/html",
	".js":    "text/javascript",
	".json":  "application/json",
	".md":    "text/markdown",
	".mod":   "text/plain",
	".proto": "text/plain",
	".py":    "text/x-python",
	".rb":    "text/x-ruby",
	".sh":    "text/x-shellscript",
	".sql":   "text/plain",
	".sum":   "text/plain",
	".toml":  "text/plain",
	".ts":    "text/plain",
	".txt":   "text/plain",
	".yaml":  "application/x-yaml",
	".yml":   "application/x-yaml",
	".zsh":   "text/x-shellscript",
}

func GuessFileType(filePath string, content []byte) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext != "" {
		if filetype, ok := extensionFiletypes[ext]; ok {
			return filetype
		}
	}

	if len(content) == 0 {
		return "unknown"
	}

	sniff := content
	if len(sniff) > 512 {
		sniff = sniff[:512]
	}

	filetype := http.DetectContentType(sniff)
	if filetype == "" {
		return "unknown"
	}
	if base, _, ok := strings.Cut(filetype, ";"); ok {
		filetype = strings.TrimSpace(base)
	}
	return filetype
}
