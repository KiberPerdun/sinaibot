package main

import (
	"context"
	"encoding/json"
	"log"
	"time"
)

type UpdateHandler struct {
	client      *TelegramClient
	messageChan chan MessageEvent
	inlineChan  chan InlineEvent
	stopChan    chan struct{}
}

type MessageEvent struct {
	UserID int64
	Text   string
}

type InlineEvent struct {
	QueryID string
	UserID  int64
	Query   string
}

func NewUpdateHandler(client *TelegramClient) *UpdateHandler {
	return &UpdateHandler{
		client:      client,
		messageChan: make(chan MessageEvent, 100),
		inlineChan:  make(chan InlineEvent, 100),
		stopChan:    make(chan struct{}),
	}
}

func (h *UpdateHandler) Start(ctx context.Context) {
	go h.pollUpdates(ctx)
}

func (h *UpdateHandler) Stop() {
	close(h.stopChan)
}

func (h *UpdateHandler) GetMessageChannel() <-chan MessageEvent {
	return h.messageChan
}

func (h *UpdateHandler) GetInlineChannel() <-chan InlineEvent {
	return h.inlineChan
}

func (h *UpdateHandler) pollUpdates(ctx context.Context) {
	offset := int64(0)
	backoff := time.Second

	for {
		select {
		case <-h.stopChan:
			return
		case <-ctx.Done():
			return
		default:
			updates, err := h.client.GetUpdates(ctx, offset)
			if err != nil {
				log.Printf("Ошибка при получении обновлений: %v", err)

				if ctx.Err() != nil {
					log.Println("Контекст был отменен, останавливаем обработку обновлений")
					return
				}

				time.Sleep(backoff)
				if backoff < 30*time.Second {
					backoff *= 2
				}
				continue
			}

			backoff = time.Second

			if len(updates) > 0 {
				for _, update := range updates {
					updateJSON, _ := json.MarshalIndent(update, "", "  ")
					log.Printf("Получено новое обновление:\n%s", string(updateJSON))

					// Обрабатываем обычные сообщения
					if update.Message.Text != "" {
						h.messageChan <- MessageEvent{
							UserID: update.Message.Chat.ID,
							Text:   update.Message.Text,
						}
					}

					// Обрабатываем inline запросы
					if update.InlineQuery != nil {
						log.Printf("Получен inline запрос: %s от пользователя %d",
							update.InlineQuery.Query, update.InlineQuery.From.ID)

						h.inlineChan <- InlineEvent{
							QueryID: update.InlineQuery.ID,
							UserID:  update.InlineQuery.From.ID,
							Query:   update.InlineQuery.Query,
						}
					}

					if update.UpdateID >= offset {
						offset = update.UpdateID + 1
					}
				}
			}
		}
	}
}
