// Package model contains the post domain entities.
package model

import "time"

type Post struct {
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	Content    string    `json:"content"`
	AuthorID   string    `json:"author_id"`
	AuthorName string    `json:"author_name,omitempty"`
	Likes      int       `json:"likes"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// UserInfo is the subset of user data the post service fetches over
// gRPC to display author details without owning user data.
type UserInfo struct {
	ID    string
	Name  string
	Email string
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

type PostCreatedEvent struct {
	UserID string `json:"user_id"`
	PostID string `json:"post_id"`
	Title  string `json:"title"`
}

type PostLikedEvent struct {
	UserID  string `json:"user_id"`
	PostID  string `json:"post_id"`
	LikerID string `json:"liker_id"`
}
