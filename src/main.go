package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type UserSession struct {
	userID       int64
	messageChan  chan string
	lastActivity time.Time
	client       *TelegramClient
}

func NewUserSession(userID int64, client *TelegramClient) *UserSession {
	return &UserSession{
		userID:       userID,
		messageChan:  make(chan string, 10),
		client:       client,
		lastActivity: time.Now(),
	}
}

func (s *UserSession) Start(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	fmt.Printf("Начата сессия для пользователя %d\n", s.userID)

	for {
		select {
		case <-ctx.Done():
			fmt.Printf("Завершение сессии пользователя %d по контексту\n", s.userID)
			return

		case message, ok := <-s.messageChan:
			if !ok {
				fmt.Printf("Канал сообщений закрыт для пользователя %d\n", s.userID)
				return
			}

			s.lastActivity = time.Now()
			fmt.Printf("Получено сообщение от пользователя %d: %s\n", s.userID, message)

			err := s.client.SendMessage(ctx, s.userID, message)
			if err != nil {
				// Улучшенный вывод ошибок с рекомендациями
				log.Printf("Ошибка отправки сообщения пользователю %d: %v\n", s.userID, err)

				// Добавляем рекомендации по типичным ошибкам
				errStr := strings.ToLower(err.Error())
				if strings.Contains(errStr, "bad request") && strings.Contains(errStr, "markdown") {
					log.Printf("Совет: Проблема с разбором Markdown. Проверьте синтаксис или отключите ParseMode в SendMessage")
				} else if strings.Contains(errStr, "forbidden") {
					log.Printf("Совет: Бот не может отправить сообщение - пользователь мог заблокировать бота или удалить чат")
				} else if strings.Contains(errStr, "too many requests") {
					log.Printf("Совет: Превышены ограничения API Telegram. Сократите частоту запросов")
				}
			}
		}
	}
}

type SessionManager struct {
	sessions     map[int64]*UserSession
	client       *TelegramClient
	sessionMutex sync.Mutex
}

func NewSessionManager(client *TelegramClient) *SessionManager {
	return &SessionManager{
		sessions: make(map[int64]*UserSession),
		client:   client,
	}
}

func (m *SessionManager) ProcessMessage(ctx context.Context, wg *sync.WaitGroup, userID int64, text string) {
	m.sessionMutex.Lock()
	defer m.sessionMutex.Unlock()

	session, exists := m.sessions[userID]
	if !exists {
		session = NewUserSession(userID, m.client)
		m.sessions[userID] = session

		wg.Add(1)
		go session.Start(ctx, wg)
	}

	session.messageChan <- text
}

func (m *SessionManager) CloseAllSessions() {
	m.sessionMutex.Lock()
	defer m.sessionMutex.Unlock()

	for userID, session := range m.sessions {
		fmt.Printf("Закрытие сессии пользователя %d\n", userID)
		close(session.messageChan)
		delete(m.sessions, userID)
	}
}

func processInlineCommand(query string) (string, string) {
	query = strings.TrimSpace(query)

	if strings.HasPrefix(query, "/time") {
		now := time.Now().Format("15:04:05")
		return "Текущее время", fmt.Sprintf("Сейчас: *%s*", now)

	} else if strings.HasPrefix(query, "/date") {
		now := time.Now().Format("02.01.2006")
		return "Текущая дата", fmt.Sprintf("Сегодня: *%s*", now)

	} else if strings.HasPrefix(query, "/bold ") {
		text := strings.TrimPrefix(query, "/bold ")
		return "Жирный текст", fmt.Sprintf("*%s*", text)

	} else if strings.HasPrefix(query, "/italic ") {
		text := strings.TrimPrefix(query, "/italic ")
		return "Курсивный текст", fmt.Sprintf("_%s_", text)

	} else if strings.HasPrefix(query, "/help") || query == "" {
		return "Справка по командам",
			"*Доступные inline команды:*\n" +
				"- `/time` - показать текущее время\n" +
				"- `/date` - показать текущую дату\n" +
				"- `/bold текст` - выделить текст жирным\n" +
				"- `/italic текст` - выделить текст курсивом\n" +
				"- `/help` - показать справку"
	}

	return "Эхо", query
}

func main() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	prodToken, _ := GetBotTokens()
	log.Println("API токены успешно загружены из переменных окружения")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := NewTelegramClient(prodToken)

	updateHandler := NewUpdateHandler(client)
	updateHandler.Start(ctx)

	sessionManager := NewSessionManager(client)

	var wg sync.WaitGroup

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	// Обработка обычных сообщений
	go func() {
		for {
			select {
			case <-ctx.Done():
				return

			case msg, ok := <-updateHandler.GetMessageChannel():
				if !ok {
					return
				}
				sessionManager.ProcessMessage(ctx, &wg, msg.UserID, msg.Text)
			}
		}
	}()

	// Обработка inline запросов
	go func() {
		for {
			select {
			case <-ctx.Done():
				return

			case inlineQuery, ok := <-updateHandler.GetInlineChannel():
				if !ok {
					return
				}

				var results []InlineQueryResult
				query := strings.TrimSpace(inlineQuery.Query)

				title, resultText := processInlineCommand(query)

				article := InlineQueryResultArticle{
					Type:        "article",
					ID:          "result",
					Title:       title,
					Description: "Нажмите, чтобы отправить",
					InputMessage: InputMessageContent{
						MessageText: resultText,
						ParseMode:   "Markdown",
					},
				}
				results = append(results, article)

				if !strings.HasPrefix(query, "/") && query != "" {
					for i := 1; i <= 2; i++ {
						resultID := strconv.Itoa(i)
						echoArticle := InlineQueryResultArticle{
							Type:        "article",
							ID:          resultID,
							Title:       fmt.Sprintf("Эхо %d: %s", i, query),
							Description: "Нажмите, чтобы отправить это сообщение",
							InputMessage: InputMessageContent{
								MessageText: fmt.Sprintf("Эхо %d: %s", i, query),
								ParseMode:   "Markdown",
							},
						}
						results = append(results, echoArticle)
					}
				}

				err := client.AnswerInline(ctx, inlineQuery.QueryID, results)
				if err != nil {
					log.Printf("Ошибка ответа на inline запрос: %v", err)
				}
			}
		}
	}()

	<-sigCh
	fmt.Println("\nПолучен сигнал завершения. Закрываемся...")

	cancel()

	updateHandler.Stop()

	sessionManager.CloseAllSessions()

	wg.Wait()
	fmt.Println("Бот успешно остановлен")
}
