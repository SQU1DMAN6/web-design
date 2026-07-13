package service

import (
	"encoding/json"
	"inkdrop/config"
	"inkdrop/model"
)

// ActivityService handles activity logging and retrieval.
type ActivityService struct{}

// NewActivityService creates a new ActivityService.
func NewActivityService() *ActivityService {
	return &ActivityService{}
}

// LogEvent logs an activity event.
func (s *ActivityService) LogEvent(dropID, eventType, targetType, targetID string, userID int64, metadata map[string]interface{}) error {
	db := config.GetDB()

	metaJSON := ""
	if len(metadata) > 0 {
		b, err := json.Marshal(metadata)
		if err == nil {
			metaJSON = string(b)
		}
	}

	log := &model.ActivityLog{
		DropID:     dropID,
		UserID:     userID,
		EventType:  eventType,
		TargetType: targetType,
		TargetID:   targetID,
		Metadata:   metaJSON,
	}

	return model.LogActivity(db, log)
}

// GetDropActivity returns activity for a drop.
func (s *ActivityService) GetDropActivity(dropID string, limit, offset int) ([]*model.ActivityLog, error) {
	db := config.GetDB()
	return model.GetDropActivity(db, dropID, limit, offset)
}

// GetUserActivity returns activity for a user.
func (s *ActivityService) GetUserActivity(userID int64, limit, offset int) ([]*model.ActivityLog, error) {
	db := config.GetDB()
	return model.GetUserActivity(db, userID, limit, offset)
}
