// Package model contains the post domain entities.
package model

import "time"

type Post struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	AuthorID  string    `json:"author_id"`
	Likes     int       `json:"likes"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CreatePostInput struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

// UpdatePostInput carries optional fields for a partial update; nil
// means "not provided" so absent and empty values stay distinguishable.
type UpdatePostInput struct {
	Title   *string `json:"title,omitempty"`
	Content *string `json:"content,omitempty"`
}
