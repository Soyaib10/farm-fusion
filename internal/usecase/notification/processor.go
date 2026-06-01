package notification

import (
	"context"
	"fmt"
	"time"

	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/google/uuid"
)

type ProcessorUseCase interface {
	Process(ctx context.Context, payload *domain.NotificationPayload) error
}

type processorUseCase struct {
	emailSender EmailSender
	logs        LogRepository
	now         func() time.Time
}

func NewProcessorUseCase(emailSender EmailSender, logs LogRepository) ProcessorUseCase {
	return &processorUseCase{
		emailSender: emailSender,
		logs:        logs,
		now:         func() time.Time { return time.Now().UTC() },
	}
}

func (uc *processorUseCase) Process(ctx context.Context, payload *domain.NotificationPayload) error {
	content, sendErr := uc.emailSender.SendWeatherNotification(ctx, payload)

	logEntry := &domain.NotificationLog{
		ID:               uuid.New(),
		FarmID:           payload.FarmID,
		UserID:           payload.UserID,
		NotificationType: "scheduled",
		AlertCount:       len(payload.Alerts),
		EmailSent:        sendErr == nil,
		EmailContent:     content,
		SentAt:           uc.now(),
	}
	if sendErr != nil {
		logEntry.ErrorMessage = sendErr.Error()
	}

	if err := uc.logs.Create(ctx, logEntry); err != nil && sendErr != nil {
		return fmt.Errorf("send email: %v; log notification: %w", sendErr, err)
	}

	if sendErr != nil {
		return sendErr
	}
	return nil
}
