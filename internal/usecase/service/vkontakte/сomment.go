package vkontakte

import (
	"fmt"
	"github.com/SevereCloud/vksdk/v3/api"
	"github.com/labstack/gommon/log"
	"postic-backend/internal/entity"
	"postic-backend/internal/repo"
	"postic-backend/internal/usecase"
	"sync"
)

type Comment struct {
	commentRepo repo.Comment
	teamRepo    repo.Team
	uploadRepo  repo.Upload
	subscribers map[entity.Subscriber]chan *entity.CommentEvent
	mu          sync.Mutex
}

func NewVkontakteComment(
	commentRepo repo.Comment,
	teamRepo repo.Team,
	uploadRepo repo.Upload,
) usecase.CommentActionPlatform {
	return &Comment{
		commentRepo: commentRepo,
		teamRepo:    teamRepo,
		uploadRepo:  uploadRepo,
		subscribers: make(map[entity.Subscriber]chan *entity.CommentEvent),
	}
}

func (c *Comment) SubscribeToCommentEvents(userID, teamID, postUnionID int) <-chan *entity.CommentEvent {
	// Create subscriber entity
	sub := entity.Subscriber{
		UserID:      userID,
		TeamID:      teamID,
		PostUnionID: postUnionID,
	}

	// Lock access to subscribers map for thread safety
	c.mu.Lock()
	defer c.mu.Unlock()

	// If subscriber already exists, return existing channel
	if ch, ok := c.subscribers[sub]; ok {
		return ch
	}

	// Create new channel for subscriber
	ch := make(chan *entity.CommentEvent)
	c.subscribers[sub] = ch
	return ch
}

func (c *Comment) UnsubscribeFromComments(userID, teamID, postUnionID int) {
	sub := entity.Subscriber{
		UserID:      userID,
		TeamID:      teamID,
		PostUnionID: postUnionID,
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if ch, ok := c.subscribers[sub]; ok {
		close(ch)
		delete(c.subscribers, sub)
	}
}

func (c *Comment) ReplyComment(request *entity.ReplyCommentRequest) (int, error) {
	return 0, nil
}

func (c *Comment) DeleteComment(request *entity.DeleteCommentRequest) error {
	// Get comment information
	comment, err := c.commentRepo.GetCommentInfo(request.PostCommentID)
	if err != nil {
		return err
	}

	// Get VK credentials
	groupID, adminAPIKey, _, err := c.teamRepo.GetVKCredsByTeamID(request.TeamID)
	if err != nil {
		return err
	}

	// Initialize VK API client
	vk := api.NewVK(adminAPIKey)

	// Delete comment via VK API
	params := api.Params{
		"owner_id":   -groupID, // Negative sign for community ID
		"comment_id": comment.CommentPlatformID,
	}

	_, err = vk.WallDeleteComment(params)
	if err != nil {
		return fmt.Errorf("failed to delete VK comment: %w", err)
	}

	// Delete comment from database
	err = c.commentRepo.DeleteComment(request.PostCommentID)
	if err != nil {
		log.Errorf("Failed to delete comment from database: %v", err)
		return err
	}

	// Notify subscribers about deleted comment
	postUnionId := 0
	if comment.PostUnionID != nil {
		postUnionId = *comment.PostUnionID
	}
	err = c.notifySubscribers(request.PostCommentID, postUnionId, request.TeamID, "deleted")
	if err != nil {
		log.Errorf("Failed to notify subscribers about deleted comment: %v", err)
	}

	return nil
}

// notifySubscribers sends notifications to subscribers about comment events
func (c *Comment) notifySubscribers(commentID, postUnionID, teamID int, eventType string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Get team members
	teamMemberIDs, err := c.teamRepo.GetTeamUsers(teamID)
	if err != nil {
		log.Errorf("Failed to get team members: %v", err)
		return err
	}

	for _, memberID := range teamMemberIDs {
		// Notify subscribers for team-level events
		sub := entity.Subscriber{
			UserID:      memberID,
			TeamID:      teamID,
			PostUnionID: 0,
		}
		if ch, ok := c.subscribers[sub]; ok {
			go func() {
				ch <- &entity.CommentEvent{
					CommentID: commentID,
					Type:      eventType,
				}
			}()
		}

		// Notify subscribers for post-level events
		if postUnionID != 0 {
			sub.PostUnionID = postUnionID
			if ch, ok := c.subscribers[sub]; ok {
				go func() {
					ch <- &entity.CommentEvent{
						CommentID: commentID,
						Type:      eventType,
					}
				}()
			}
		}
	}
	return nil
}
