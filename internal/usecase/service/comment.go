package service

import (
	"bytes"
	"encoding/json"
	"github.com/labstack/gommon/log"
	"net/http"
	"postic-backend/internal/entity"
	"postic-backend/internal/repo"
	"postic-backend/internal/usecase"
)

type Comment struct {
	commentRepo repo.Comment
	mlURL       string
}

func NewComment(commentRepo repo.Comment, mlURL string) usecase.Comment {
	return &Comment{
		commentRepo: commentRepo,
		mlURL:       mlURL,
	}
}

func (c Comment) GetLastComments(postUnionID int, limit int) ([]*entity.JustTextComment, error) {
	//TODO implement me
	panic("implement me")
}

func (c Comment) GetSummarize(postUnionID int) (*entity.Summarize, error) {
	comments, err := c.commentRepo.GetLastComments(postUnionID, 100)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Comments []string `json:"comments"`
	}
	for _, comment := range comments {
		if comment.Text == "" {
			continue
		}
		payload.Comments = append(payload.Comments, comment.Text)
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	log.Infof("request: %v", payload)
	req, err := http.NewRequest("GET", c.mlURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	type ServerAnswer struct {
		Response string `json:"response"`
	}

	var serverAnswer ServerAnswer
	err = json.NewDecoder(resp.Body).Decode(&serverAnswer)
	if err != nil {
		return nil, err
	}
	log.Infof("response: %v", serverAnswer)
	return &entity.Summarize{
		Markdown:    serverAnswer.Response,
		PostUnionID: postUnionID,
	}, nil
}
