package template

import (
	"fmt"
	"strings"
	"sync"
)

type FileType string

const (
	FileTypeGo         FileType = "go"
	FileTypeTypeScript FileType = "typescript"
	FileTypePython     FileType = "python"
	FileTypeHTML       FileType = "html"
	FileTypeCSS        FileType = "css"
	FileTypeJSON       FileType = "json"
	FileTypeYAML       FileType = "yaml"
	FileTypeDockerfile FileType = "dockerfile"
	FileTypeMarkdown   FileType = "markdown"
)

type GeneratedFile struct {
	Path    string   `json:"path"`
	Content string   `json:"content"`
	Type    FileType `json:"type"`
}

type ProjectSpec struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Language    string            `json:"language"`
	Features    []string          `json:"features"`
	Config      map[string]string `json:"config"`
}

type Engine struct {
	mu        sync.RWMutex
	templates map[string]TemplateFunc
}

type TemplateFunc func(spec ProjectSpec) []GeneratedFile

func New() *Engine {
	e := &Engine{
		templates: make(map[string]TemplateFunc),
	}
	e.registerDefaults()
	return e
}

func (e *Engine) registerDefaults() {
	e.Register("api-go", generateGoAPI)
	e.Register("api-typescript", generateTSAPI)
	e.Register("api-python", generatePythonAPI)
	e.Register("static-html", generateStaticHTML)
}

func (e *Engine) Register(name string, fn TemplateFunc) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.templates[name] = fn
}

func (e *Engine) Generate(templateName string, spec ProjectSpec) ([]GeneratedFile, error) {
	e.mu.RLock()
	fn, ok := e.templates[templateName]
	e.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("template '%s' not found", templateName)
	}
	return fn(spec), nil
}

func (e *Engine) Templates() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	var names []string
	for name := range e.templates {
		names = append(names, name)
	}
	return names
}

func generateGoAPI(spec ProjectSpec) []GeneratedFile {
	pkgName := strings.ToLower(strings.ReplaceAll(spec.Name, "-", ""))

	return []GeneratedFile{
		{
			Path: "main.go",
			Type: FileTypeGo,
			Content: fmt.Sprintf(`package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Welcome to %%s", "%s")
	})
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `+"`"+`{"status": "healthy", "service": "%%s"}`+"`"+`, "%s")
	})
	log.Printf("%%s starting on :8080", "%s")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
`, spec.Name, spec.Name, spec.Name),
		},
		{
			Path:    "go.mod",
			Type:    FileTypeGo,
			Content: fmt.Sprintf("module %s\n\ngo 1.22\n", pkgName),
		},
		{
			Path: "Dockerfile",
			Type: FileTypeDockerfile,
			Content: `FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o server .

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/server .
EXPOSE 8080
CMD ["./server"]
`,
		},
		{
			Path:    "README.md",
			Type:    FileTypeMarkdown,
			Content: fmt.Sprintf("# %s\n\n%s\n\n## Run\n\n```bash\ngo run .\n```\n", spec.Name, spec.Description),
		},
	}
}

func generateTSAPI(spec ProjectSpec) []GeneratedFile {
	return []GeneratedFile{
		{
			Path: "package.json",
			Type: FileTypeJSON,
			Content: fmt.Sprintf(`{
  "name": "%s",
  "version": "1.0.0",
  "description": "%s",
  "main": "dist/index.js",
  "scripts": {
    "build": "tsc",
    "start": "node dist/index.js",
    "dev": "ts-node src/index.ts"
  },
  "dependencies": {
    "express": "^4.18.0"
  },
  "devDependencies": {
    "typescript": "^5.4.0",
    "@types/express": "^4.17.0",
    "ts-node": "^10.9.0"
  }
}`, spec.Name, spec.Description),
		},
		{
			Path: "src/index.ts",
			Type: FileTypeTypeScript,
			Content: fmt.Sprintf(`import express from 'express';

const app = express();
const PORT = process.env.PORT || 3000;

app.get('/', (req, res) => {
  res.json({ message: 'Welcome to %s' });
});

app.get('/health', (req, res) => {
  res.json({ status: 'healthy', service: '%s' });
});

app.listen(PORT, () => {
  console.log('%s running on port ' + PORT);
});
`, spec.Name, spec.Name, spec.Name),
		},
		{
			Path:    "tsconfig.json",
			Type:    FileTypeJSON,
			Content: `{"compilerOptions":{"target":"ES2020","module":"commonjs","outDir":"./dist","rootDir":"./src","strict":true,"esModuleInterop":true},"include":["src/**/*"]}`,
		},
	}
}

func generatePythonAPI(spec ProjectSpec) []GeneratedFile {
	return []GeneratedFile{
		{
			Path: "app.py",
			Type: FileTypePython,
			Content: fmt.Sprintf(`from flask import Flask, jsonify

app = Flask(__name__)

@app.route('/')
def index():
    return jsonify({"message": "Welcome to %s"})

@app.route('/health')
def health():
    return jsonify({"status": "healthy", "service": "%s"})

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=5000)
`, spec.Name, spec.Name),
		},
		{
			Path:    "requirements.txt",
			Type:    FileTypePython,
			Content: "flask>=3.0\ngunicorn>=21.0\n",
		},
		{
			Path: "Dockerfile",
			Type: FileTypeDockerfile,
			Content: "FROM python:3.12-slim\nWORKDIR /app\nCOPY requirements.txt .\nRUN pip install -r requirements.txt\nCOPY . .\nEXPOSE 5000\nCMD [\"gunicorn\", \"-b\", \"0.0.0.0:5000\", \"app:app\"]\n",
		},
	}
}

func generateStaticHTML(spec ProjectSpec) []GeneratedFile {
	return []GeneratedFile{
		{
			Path: "index.html",
			Type: FileTypeHTML,
			Content: fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>%s</title>
    <link rel="stylesheet" href="style.css">
</head>
<body>
    <header><h1>%s</h1><p>%s</p></header>
    <main><p>Built with Immortal Engine</p></main>
    <script src="app.js"></script>
</body>
</html>`, spec.Name, spec.Name, spec.Description),
		},
		{
			Path: "style.css",
			Type: FileTypeCSS,
			Content: `* { margin: 0; padding: 0; box-sizing: border-box; }
body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif; line-height: 1.6; color: #333; max-width: 1200px; margin: 0 auto; padding: 2rem; }
header { text-align: center; padding: 4rem 0; }
h1 { font-size: 3rem; margin-bottom: 1rem; }`,
		},
	}
}
