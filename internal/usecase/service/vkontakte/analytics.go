package vkontakte

import (
	"fmt"
	"github.com/SevereCloud/vksdk/v3/api"
	"postic-backend/internal/entity"
	"postic-backend/internal/repo"
	"postic-backend/internal/usecase"
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

func (a *Analytics) UpdateStat(postUnionID int) error {
	post, err := a.postRepo.GetPostUnion(postUnionID)
	if err != nil {
		return fmt.Errorf("failed to get post by ID: %w", err)
	}

	postPlatform, err := a.postRepo.GetPostPlatform(postUnionID, "vk")
	if err != nil {
		return fmt.Errorf("failed to get VK post platform: %w", err)
	}

	vkChannel, err := a.teamRepo.GetVKCredsByTeamID(post.TeamID)
	if err != nil {
		return fmt.Errorf("failed to get VK credentials: %w", err)
	}

	vk := api.NewVK(vkChannel.AdminAPIKey)
	params := api.Params{
		"posts":              fmt.Sprintf("-%d_%d", vkChannel.GroupID, postPlatform.PostId),
		"extended":           1,
		"fields":             "views",
		"copy_history_depth": 0,
	}
	response, err := vk.WallGetByID(params)
	if err != nil {
		return fmt.Errorf("failed to get VK post stats: %w", err)
	}
	if len(response.Items) == 0 {
		return fmt.Errorf("post not found in VK")
	}

	stats := &entity.PostPlatformStats{
		TeamID:      post.TeamID,
		PostUnionID: postUnionID,
		Platform:    "vk",
		Views:       response.Items[0].Views.Count,
		Comments:    response.Items[0].Comments.Count,
		Reactions:   response.Items[0].Likes.Count,
	}

	err = a.analyticsRepo.UpdateLastPlatformStats(stats, "vk")
	if err != nil {
		return fmt.Errorf("failed to update post platform stats: %w", err)
	}

	return nil
}
