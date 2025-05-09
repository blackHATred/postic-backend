package vkontakte

import (
	"errors"
	"fmt"
	"github.com/SevereCloud/vksdk/v3/api"
	"postic-backend/internal/entity"
	"postic-backend/internal/repo"
	"postic-backend/internal/usecase"
	"time"
)

type Analytics struct {
	teamRepo      repo.Team
	postRepo      repo.Post
	analyticsRepo repo.Analytics
}

func NewVkontakteAnalytics(
	teamRepo repo.Team,
	postRepo repo.Post,
	analyticsRepo repo.Analytics,
) usecase.AnalyticsPlatform {
	return &Analytics{
		teamRepo:      teamRepo,
		postRepo:      postRepo,
		analyticsRepo: analyticsRepo,
	}
}

func (a *Analytics) UpdateStat(postUnionID int) (*entity.PlatformStats, error) {
	post, err := a.postRepo.GetPostUnion(postUnionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get post by ID: %w", err)
	}

	postPlatform, err := a.postRepo.GetPostPlatform(postUnionID, "vk")
	if err != nil {
		return nil, fmt.Errorf("failed to get VK post platform: %w", err)
	}

	stats, err := a.analyticsRepo.GetPostPlatformStatsByPostUnionID(postUnionID, "vk")
	switch {
	case errors.Is(err, repo.ErrPostPlatformStatsNotFound):
		// Create new stats
		stats = &entity.PostPlatformStats{
			PostUnionID: postUnionID,
			Platform:    "vk",
			TeamID:      post.TeamID,
			Views:       0,
			Comments:    0,
			Reactions:   0,
			LastUpdate:  time.Now(),
		}
		err = a.analyticsRepo.AddPostPlatformStats(stats)
		if err != nil {
			return nil, fmt.Errorf("failed to add post platform stats: %w", err)
		}
	case err != nil:
		return nil, fmt.Errorf("failed to get post platform stats: %w", err)
	}

	vkChannel, err := a.teamRepo.GetVKCredsByTeamID(post.TeamID)
	if err != nil {
		return nil, fmt.Errorf("failed to get VK credentials: %w", err)
	}

	vk := api.NewVK(vkChannel.AdminAPIKey)

	// Query VK API for post stats
	params := api.Params{
		"posts":              fmt.Sprintf("-%d_%d", vkChannel.GroupID, postPlatform.PostId),
		"extended":           1,
		"fields":             "views",
		"copy_history_depth": 0,
	}

	response, err := vk.WallGetByID(params)
	if err != nil {
		return nil, fmt.Errorf("failed to get VK post stats: %w", err)
	}

	if len(response.Items) == 0 {
		return nil, fmt.Errorf("post not found in VK")
	}

	stats.Views = response.Items[0].Views.Count
	stats.Comments = response.Items[0].Comments.Count
	stats.Reactions = response.Items[0].Likes.Count
	stats.LastUpdate = time.Now()

	// Update statistics in database
	err = a.analyticsRepo.EditPostPlatformStats(stats)
	if err != nil {
		return nil, fmt.Errorf("failed to update post platform stats: %w", err)
	}

	return &entity.PlatformStats{
		Views:     stats.Views,
		Comments:  stats.Comments,
		Reactions: stats.Reactions,
	}, nil
}
