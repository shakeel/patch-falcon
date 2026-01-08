package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"srv.exe.dev/db"
	"srv.exe.dev/db/dbgen"
)

// WikiSummary represents the response from Wikipedia's summary API
type WikiSummary struct {
	Title       string `json:"title"`
	Extract     string `json:"extract"`
	Description string `json:"description"`
	ContentURLs struct {
		Desktop struct {
			Page string `json:"page"`
		} `json:"desktop"`
	} `json:"content_urls"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	// Fetch random Wikipedia article summary
	summary, err := fetchRandomWikiSummary()
	if err != nil {
		return fmt.Errorf("fetch wiki summary: %w", err)
	}

	fmt.Printf("Found article: %s\n", summary.Title)

	// Connect to database
	wdb, err := db.Open("db.sqlite3")
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer wdb.Close()

	// Create the blog post
	if err := createPost(wdb, summary); err != nil {
		return fmt.Errorf("create post: %w", err)
	}

	fmt.Println("Post created successfully!")
	return nil
}

func fetchRandomWikiSummary() (*WikiSummary, error) {
	// Wikipedia REST API for random article summary
	url := "https://en.wikipedia.org/api/rest_v1/page/random/summary"

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	// Wikipedia API requires a User-Agent
	req.Header.Set("User-Agent", "CitizenOfTheWorldBot/1.0 (https://patch-falcon.exe.xyz:8000)")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("wikipedia API returned status %d", resp.StatusCode)
	}

	var summary WikiSummary
	if err := json.NewDecoder(resp.Body).Decode(&summary); err != nil {
		return nil, err
	}

	return &summary, nil
}

func createPost(wdb *sql.DB, summary *WikiSummary) error {
	q := dbgen.New(wdb)

	// Generate slug from title
	slug := generateSlug(summary.Title)
	// Add date to make it unique
	dateStr := time.Now().Format("2006-01-02")
	slug = fmt.Sprintf("wiki-%s-%s", dateStr, slug)

	// Build content
	var content strings.Builder
	content.WriteString(fmt.Sprintf("Today's random Wikipedia discovery: **%s**\n\n", summary.Title))
	
	if summary.Description != "" {
		content.WriteString(fmt.Sprintf("*%s*\n\n", summary.Description))
	}
	
	content.WriteString(summary.Extract)
	content.WriteString("\n\n")
	content.WriteString(fmt.Sprintf("Read more on Wikipedia: %s", summary.ContentURLs.Desktop.Page))

	// Create the post using context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	_, err := q.CreatePost(ctx, dbgen.CreatePostParams{
		Slug:      slug,
		Title:     fmt.Sprintf("Wiki Discovery: %s", summary.Title),
		Content:   content.String(),
		Published: 1,
	})

	return err
}

func generateSlug(title string) string {
	// Convert to lowercase
	slug := strings.ToLower(title)
	// Replace spaces with hyphens
	slug = strings.ReplaceAll(slug, " ", "-")
	// Remove non-alphanumeric characters except hyphens
	reg := regexp.MustCompile(`[^a-z0-9-]`)
	slug = reg.ReplaceAllString(slug, "")
	// Remove multiple consecutive hyphens
	reg = regexp.MustCompile(`-+`)
	slug = reg.ReplaceAllString(slug, "-")
	// Trim hyphens from ends
	slug = strings.Trim(slug, "-")
	// Limit length
	if len(slug) > 50 {
		slug = slug[:50]
	}
	return slug
}
