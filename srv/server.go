package srv

import (
	"database/sql"
	"html/template"
	"log/slog"
	"net/http"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"srv.exe.dev/db"
	"srv.exe.dev/db/dbgen"
)

type Server struct {
	DB           *sql.DB
	Hostname     string
	TemplatesDir string
	StaticDir    string
	templates    *template.Template
}

type PostView struct {
	ID          int64
	Slug        string
	Title       string
	Content     string
	Excerpt     string
	ContentHTML template.HTML
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func New(dbPath, hostname string) (*Server, error) {
	_, thisFile, _, _ := runtime.Caller(0)
	baseDir := filepath.Dir(thisFile)
	srv := &Server{
		Hostname:     hostname,
		TemplatesDir: filepath.Join(baseDir, "templates"),
		StaticDir:    filepath.Join(baseDir, "static"),
	}
	if err := srv.setUpDatabase(dbPath); err != nil {
		return nil, err
	}
	if err := srv.loadTemplates(); err != nil {
		return nil, err
	}
	return srv, nil
}

func (s *Server) loadTemplates() error {
	tmpl, err := template.ParseGlob(filepath.Join(s.TemplatesDir, "*.html"))
	if err != nil {
		return err
	}
	s.templates = tmpl
	return nil
}

func (s *Server) render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, name, data); err != nil {
		slog.Error("render template", "name", name, "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (s *Server) HandleHome(w http.ResponseWriter, r *http.Request) {
	q := dbgen.New(s.DB)
	dbPosts, err := q.GetPublishedPosts(r.Context())
	if err != nil {
		slog.Error("get posts", "error", err)
	}

	posts := make([]PostView, 0, len(dbPosts))
	for _, p := range dbPosts {
		posts = append(posts, PostView{
			ID:        p.ID,
			Slug:      p.Slug,
			Title:     p.Title,
			Excerpt:   excerpt(p.Content, 200),
			CreatedAt: p.CreatedAt,
		})
	}

	s.render(w, "base.html", map[string]any{
		"Posts": posts,
		"Year":  time.Now().Year(),
		"Page":  "home",
	})
}

func (s *Server) HandlePost(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	q := dbgen.New(s.DB)
	p, err := q.GetPostBySlug(r.Context(), slug)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if p.Published == 0 {
		http.NotFound(w, r)
		return
	}

	post := PostView{
		ID:          p.ID,
		Slug:        p.Slug,
		Title:       p.Title,
		Content:     p.Content,
		ContentHTML: renderContent(p.Content),
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
	}

	s.render(w, "base.html", map[string]any{
		"Post": post,
		"Year": time.Now().Year(),
		"Page": "post",
	})
}

func (s *Server) HandleArchive(w http.ResponseWriter, r *http.Request) {
	q := dbgen.New(s.DB)
	dbPosts, err := q.GetPublishedPosts(r.Context())
	if err != nil {
		slog.Error("get posts", "error", err)
	}

	posts := make([]PostView, 0, len(dbPosts))
	for _, p := range dbPosts {
		posts = append(posts, PostView{
			Slug:      p.Slug,
			Title:     p.Title,
			CreatedAt: p.CreatedAt,
		})
	}

	s.render(w, "base.html", map[string]any{
		"Posts": posts,
		"Year":  time.Now().Year(),
		"Page":  "archive",
	})
}

func (s *Server) setUpDatabase(dbPath string) error {
	wdb, err := db.Open(dbPath)
	if err != nil {
		return err
	}
	s.DB = wdb
	if err := db.RunMigrations(wdb); err != nil {
		return err
	}
	return nil
}

func (s *Server) Serve(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.HandleHome)
	mux.HandleFunc("GET /post/{slug}", s.HandlePost)
	mux.HandleFunc("GET /archive", s.HandleArchive)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(s.StaticDir))))
	slog.Info("starting server", "addr", addr)
	return http.ListenAndServe(addr, mux)
}

// Helper functions

func excerpt(content string, maxLen int) string {
	// Strip any HTML-like content for excerpt
	content = strings.TrimSpace(content)
	if len(content) <= maxLen {
		return content
	}
	// Find last space before maxLen
	truncated := content[:maxLen]
	if idx := strings.LastIndex(truncated, " "); idx > 0 {
		truncated = truncated[:idx]
	}
	return truncated + "..."
}

func renderContent(content string) template.HTML {
	// Simple markdown-like rendering
	lines := strings.Split(content, "\n")
	var sb strings.Builder
	var inCodeBlock bool
	var inList bool
	var paragraph []string

	flushParagraph := func() {
		if len(paragraph) > 0 {
			text := strings.Join(paragraph, " ")
			sb.WriteString("<p>")
			sb.WriteString(template.HTMLEscapeString(text))
			sb.WriteString("</p>\n")
			paragraph = nil
		}
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Code blocks (4 spaces indent)
		if strings.HasPrefix(line, "    ") && !inCodeBlock {
			flushParagraph()
			if inList {
				sb.WriteString("</ul>\n")
				inList = false
			}
			sb.WriteString("<pre><code>")
			inCodeBlock = true
		}
		if inCodeBlock {
			if strings.HasPrefix(line, "    ") {
				sb.WriteString(template.HTMLEscapeString(strings.TrimPrefix(line, "    ")))
				sb.WriteString("\n")
				continue
			} else {
				sb.WriteString("</code></pre>\n")
				inCodeBlock = false
			}
		}

		// Empty line
		if trimmed == "" {
			flushParagraph()
			if inList {
				sb.WriteString("</ul>\n")
				inList = false
			}
			continue
		}

		// Headers
		if strings.HasPrefix(trimmed, "## ") {
			flushParagraph()
			if inList {
				sb.WriteString("</ul>\n")
				inList = false
			}
			sb.WriteString("<h2>")
			sb.WriteString(template.HTMLEscapeString(strings.TrimPrefix(trimmed, "## ")))
			sb.WriteString("</h2>\n")
			continue
		}
		if strings.HasPrefix(trimmed, "# ") {
			flushParagraph()
			if inList {
				sb.WriteString("</ul>\n")
				inList = false
			}
			sb.WriteString("<h2>")
			sb.WriteString(template.HTMLEscapeString(strings.TrimPrefix(trimmed, "# ")))
			sb.WriteString("</h2>\n")
			continue
		}

		// List items
		if strings.HasPrefix(trimmed, "- ") {
			flushParagraph()
			if !inList {
				sb.WriteString("<ul>\n")
				inList = true
			}
			sb.WriteString("<li>")
			sb.WriteString(template.HTMLEscapeString(strings.TrimPrefix(trimmed, "- ")))
			sb.WriteString("</li>\n")
			continue
		}

		// Bold text **text**
		paragraph = append(paragraph, trimmed)
	}

	if inCodeBlock {
		sb.WriteString("</code></pre>\n")
	}
	if inList {
		sb.WriteString("</ul>\n")
	}
	flushParagraph()

	// Process inline formatting
	result := sb.String()
	result = processInlineFormatting(result)
	return template.HTML(result)
}

func processInlineFormatting(s string) string {
	// Bold: **text**
	for {
		start := strings.Index(s, "**")
		if start == -1 {
			break
		}
		end := strings.Index(s[start+2:], "**")
		if end == -1 {
			break
		}
		end += start + 2
		s = s[:start] + "<strong>" + s[start+2:end] + "</strong>" + s[end+2:]
	}
	// Inline code: `text`
	for {
		start := strings.Index(s, "`")
		if start == -1 {
			break
		}
		end := strings.Index(s[start+1:], "`")
		if end == -1 {
			break
		}
		end += start + 1
		s = s[:start] + "<code>" + s[start+1:end] + "</code>" + s[end+1:]
	}
	return s
}
