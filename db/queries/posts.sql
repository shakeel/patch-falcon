-- name: GetPublishedPosts :many
SELECT id, slug, title, content, created_at, updated_at
FROM posts
WHERE published = 1
ORDER BY created_at DESC;

-- name: GetPostBySlug :one
SELECT id, slug, title, content, published, created_at, updated_at
FROM posts
WHERE slug = ?;

-- name: GetAllPosts :many
SELECT id, slug, title, content, published, created_at, updated_at
FROM posts
ORDER BY created_at DESC;

-- name: CreatePost :one
INSERT INTO posts (slug, title, content, published, created_at, updated_at)
VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
RETURNING *;

-- name: UpdatePost :exec
UPDATE posts
SET title = ?, content = ?, published = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: DeletePost :exec
DELETE FROM posts WHERE id = ?;
