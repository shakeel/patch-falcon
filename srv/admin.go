package srv

import (
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"srv.exe.dev/db/dbgen"
)

// AdminEmails contains emails allowed to access admin
var AdminEmails = []string{
	// Add your email here
}

func (s *Server) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Skip auth in dev mode
		if os.Getenv("DEV_MODE") == "1" {
			next(w, r)
			return
		}
		
		email := strings.TrimSpace(r.Header.Get("X-ExeDev-Email"))
		
		// If no admin emails configured, allow any authenticated user
		if len(AdminEmails) == 0 {
			if email == "" {
				http.Redirect(w, r, "/__exe.dev/login?redirect="+r.URL.Path, http.StatusFound)
				return
			}
			next(w, r)
			return
		}
		
		// Check if email is in admin list
		for _, admin := range AdminEmails {
			if strings.EqualFold(email, admin) {
				next(w, r)
				return
			}
		}
		
		if email == "" {
			http.Redirect(w, r, "/__exe.dev/login?redirect="+r.URL.Path, http.StatusFound)
			return
		}
		
		http.Error(w, "Forbidden", http.StatusForbidden)
	}
}

func (s *Server) HandleAdminList(w http.ResponseWriter, r *http.Request) {
	q := dbgen.New(s.DB)
	posts, err := q.GetAllPosts(r.Context())
	if err != nil {
		slog.Error("get all posts", "error", err)
	}

	var postViews []PostView
	for _, p := range posts {
		postViews = append(postViews, PostView{
			ID:        p.ID,
			Slug:      p.Slug,
			Title:     p.Title,
			Published: p.Published == 1,
			CreatedAt: p.CreatedAt,
			UpdatedAt: p.UpdatedAt,
		})
	}

	s.render(w, "admin.html", map[string]any{
		"Posts": postViews,
		"Year":  time.Now().Year(),
	})
}

func (s *Server) HandleAdminNew(w http.ResponseWriter, r *http.Request) {
	s.render(w, "admin_edit.html", map[string]any{
		"IsNew": true,
		"Post":  PostView{},
		"Year":  time.Now().Year(),
	})
}

func (s *Server) HandleAdminCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	slug := strings.TrimSpace(r.FormValue("slug"))
	title := strings.TrimSpace(r.FormValue("title"))
	content := r.FormValue("content")
	published := r.FormValue("published") == "on"

	if slug == "" || title == "" {
		s.render(w, "admin_edit.html", map[string]any{
			"IsNew": true,
			"Post": PostView{
				Slug:    slug,
				Title:   title,
				Content: content,
			},
			"Error": "Slug and title are required",
			"Year":  time.Now().Year(),
		})
		return
	}

	q := dbgen.New(s.DB)
	var pub int64
	if published {
		pub = 1
	}
	_, err := q.CreatePost(r.Context(), dbgen.CreatePostParams{
		Slug:      slug,
		Title:     title,
		Content:   content,
		Published: pub,
	})
	if err != nil {
		slog.Error("create post", "error", err)
		s.render(w, "admin_edit.html", map[string]any{
			"IsNew": true,
			"Post": PostView{
				Slug:    slug,
				Title:   title,
				Content: content,
			},
			"Error": "Failed to create post: " + err.Error(),
			"Year":  time.Now().Year(),
		})
		return
	}

	http.Redirect(w, r, "/admin", http.StatusFound)
}

func (s *Server) HandleAdminEdit(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	q := dbgen.New(s.DB)
	posts, err := q.GetAllPosts(r.Context())
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	var post *dbgen.Post
	for _, p := range posts {
		if p.ID == id {
			post = &p
			break
		}
	}
	if post == nil {
		http.NotFound(w, r)
		return
	}

	s.render(w, "admin_edit.html", map[string]any{
		"IsNew": false,
		"Post": PostView{
			ID:        post.ID,
			Slug:      post.Slug,
			Title:     post.Title,
			Content:   post.Content,
			Published: post.Published == 1,
			CreatedAt: post.CreatedAt,
		},
		"Year": time.Now().Year(),
	})
}

func (s *Server) HandleAdminUpdate(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	title := strings.TrimSpace(r.FormValue("title"))
	content := r.FormValue("content")
	published := r.FormValue("published") == "on"

	q := dbgen.New(s.DB)
	var pub int64
	if published {
		pub = 1
	}
	err = q.UpdatePost(r.Context(), dbgen.UpdatePostParams{
		Title:     title,
		Content:   content,
		Published: pub,
		ID:        id,
	})
	if err != nil {
		slog.Error("update post", "error", err)
		http.Error(w, "Failed to update", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin", http.StatusFound)
}

func (s *Server) HandleAdminDelete(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	q := dbgen.New(s.DB)
	err = q.DeletePost(r.Context(), id)
	if err != nil {
		slog.Error("delete post", "error", err)
	}

	http.Redirect(w, r, "/admin", http.StatusFound)
}
