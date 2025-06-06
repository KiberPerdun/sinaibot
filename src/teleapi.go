package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"
)

const (
	APIBaseURL       = "https://api.telegram.org/bot"
	GetUpdatesPath   = "/getUpdates"
	SendMessagePath  = "/sendMessage"
	AnswerInlinePath = "/answerInlineQuery"

	LongPollTimeout = 30
	HTTPTimeout     = 60 * time.Second

	// Имена переменных окружения для токенов
	EnvProdToken  = "TELEGRAM_PRODUCTION_TOKEN"
	EnvAlertToken = "TELEGRAM_ALERT_TOKEN"
)

type TelegramError struct {
	OK          bool   `json:"ok"`
	ErrorCode   int    `json:"error_code"`
	Description string `json:"description"`
}

func (e *TelegramError) Error() string {
	return fmt.Sprintf("Telegram API ошибка %d: %s", e.ErrorCode, e.Description)
}

func GetRequiredEnv(name string) string {
	value := os.Getenv(name)
	if value == "" {
		fmt.Fprintf(os.Stderr, "ОШИБКА: Не установлена обязательная переменная окружения %s\n", name)
		os.Exit(1)
	}
	return value
}

func GetBotTokens() (productionToken, alertToken string) {
	productionToken = GetRequiredEnv(EnvProdToken)
	alertToken = GetRequiredEnv(EnvAlertToken)
	return
}

func parseAPIError(statusCode int, responseBody []byte) error {
	var telegramErr TelegramError
	if err := json.Unmarshal(responseBody, &telegramErr); err != nil {
		return fmt.Errorf("ошибка API: HTTP %d, тело: %s", statusCode, string(responseBody))
	}

	var additionalInfo string
	switch statusCode {
	case 400:
		additionalInfo = " (Неверный запрос)"
	case 401:
		additionalInfo = " (Неавторизованный запрос, проверьте токен бота)"
	case 403:
		additionalInfo = " (Запрещено: у бота нет доступа)"
	case 404:
		additionalInfo = " (Метод не найден)"
	case 409:
		additionalInfo = " (Конфликт)"
	case 429:
		additionalInfo = " (Слишком много запросов, превышен лимит)"
	case 500, 502, 503, 504:
		additionalInfo = " (Ошибка на стороне серверов Telegram)"
	}

	return fmt.Errorf("%s%s", telegramErr.Error(), additionalInfo)
}

type TelegramClient struct {
	httpClient *http.Client
	botToken   string
	apiURL     string
}

func NewTelegramClient(token string) *TelegramClient {
	return &TelegramClient{
		httpClient: &http.Client{
			Timeout: HTTPTimeout,
		},
		botToken: token,
		apiURL:   APIBaseURL + token,
	}
}

func (c *TelegramClient) GetUpdates(ctx context.Context, offset int64) ([]TelegramUpdate, error) {
	url := c.apiURL + GetUpdatesPath + fmt.Sprintf("?offset=%d&timeout=%d", offset, LongPollTimeout)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания запроса: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ошибка сетевого соединения: %w", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения ответа: %w", err)
	}

	var response TelegramUpdateResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("ошибка разбора JSON: %w", err)
	}

	if !response.Ok {
		return nil, parseAPIError(resp.StatusCode, body)
	}

	return response.Result, nil
}

func (c *TelegramClient) SendMessage(ctx context.Context, chatID int64, text string) error {
	message := OutgoingMessage{
		ChatID:                chatID,
		Text:                  text,
		ParseMode:             "Markdown",
		DisableWebPagePreview: true,
	}

	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("ошибка кодирования JSON: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiURL+SendMessagePath, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("ошибка создания запроса: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ошибка сетевого соединения: %w", err)
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return parseAPIError(resp.StatusCode, body)
	}

	var response struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(body, &response); err != nil || !response.OK {
		return errors.New("API вернул успешный HTTP код, но флаг ok=false")
	}

	return nil
}

func (c *TelegramClient) SendMessageWithButtons(ctx context.Context, chatID int64, text string, buttonLabels []string, isPersistent bool, buttonsPerRow, breakRowAfter int) error {
	var keyboard [][]Button
	var currentRow []Button

	for i, label := range buttonLabels {
		currentRow = append(currentRow, Button{Text: label})

		if len(currentRow) == buttonsPerRow || i+1 == breakRowAfter {
			keyboard = append(keyboard, currentRow)
			currentRow = []Button{}
		}
	}

	if len(currentRow) > 0 {
		keyboard = append(keyboard, currentRow)
	}

	message := OutgoingMessageWithKeyboard{
		ChatID:                chatID,
		Text:                  text,
		ParseMode:             "Markdown",
		DisableWebPagePreview: true,
		ReplyMarkup: KeyboardMarkup{
			Keyboard:       keyboard,
			ResizeKeyboard: true,
			IsPersistent:   isPersistent,
		},
	}

	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("ошибка кодирования JSON: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiURL+SendMessagePath, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("ошибка создания запроса: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ошибка сетевого соединения: %w", err)
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return parseAPIError(resp.StatusCode, body)
	}

	var response struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(body, &response); err != nil || !response.OK {
		return errors.New("API вернул успешный HTTP код, но флаг ok=false")
	}

	return nil
}

// AnswerInline отправляет ответ на inline запрос
func (c *TelegramClient) AnswerInline(ctx context.Context, queryID string, results []InlineQueryResult) error {
	answer := AnswerInlineQuery{
		InlineQueryID: queryID,
		Results:       results,
		CacheTime:     300,
		IsPersonal:    true,
	}

	data, err := json.Marshal(answer)
	if err != nil {
		return fmt.Errorf("ошибка кодирования JSON: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiURL+AnswerInlinePath, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("ошибка создания запроса: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ошибка сетевого соединения: %w", err)
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return parseAPIError(resp.StatusCode, body)
	}

	var response struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(body, &response); err != nil || !response.OK {
		return errors.New("API вернул успешный HTTP код, но флаг ok=false")
	}

	return nil
}

// Структуры для работы с API
type TelegramUpdateResponse struct {
	Ok     bool             `json:"ok"`
	Result []TelegramUpdate `json:"result"`
}

type TelegramUpdate struct {
	UpdateID    int64           `json:"update_id"`
	Message     TelegramMessage `json:"message,omitempty"`
	InlineQuery *InlineQuery    `json:"inline_query,omitempty"`
}

type TelegramMessage struct {
	MessageID int64        `json:"message_id"`
	From      TelegramUser `json:"from"`
	Chat      TelegramChat `json:"chat"`
	Date      int64        `json:"date"`
	Text      string       `json:"text"`
}

type TelegramUser struct {
	ID           int64  `json:"id"`
	IsBot        bool   `json:"is_bot"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	Username     string `json:"username"`
	LanguageCode string `json:"language_code"`
	IsPremium    bool   `json:"is_premium"`
}

type TelegramChat struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
	Type      string `json:"type"`
}

// Структуры для отправки сообщений
type OutgoingMessage struct {
	ChatID                int64  `json:"chat_id"`
	Text                  string `json:"text"`
	ParseMode             string `json:"parse_mode,omitempty"`
	DisableWebPagePreview bool   `json:"disable_web_page_preview,omitempty"`
}

type OutgoingMessageWithKeyboard struct {
	ChatID                int64          `json:"chat_id"`
	Text                  string         `json:"text"`
	ParseMode             string         `json:"parse_mode,omitempty"`
	DisableWebPagePreview bool           `json:"disable_web_page_preview,omitempty"`
	ReplyMarkup           KeyboardMarkup `json:"reply_markup"`
}

type KeyboardMarkup struct {
	Keyboard       [][]Button `json:"keyboard"`
	ResizeKeyboard bool       `json:"resize_keyboard"`
	IsPersistent   bool       `json:"is_persistent"`
}

type Button struct {
	Text string `json:"text"`
}

// Структуры для inline запросов
type InlineQuery struct {
	ID     string       `json:"id"`
	From   TelegramUser `json:"from"`
	Query  string       `json:"query"`
	Offset string       `json:"offset"`
}

type InlineQueryResult interface {
	GetType() string
}

type InlineQueryResultArticle struct {
	Type         string              `json:"type"`
	ID           string              `json:"id"`
	Title        string              `json:"title"`
	Description  string              `json:"description,omitempty"`
	ThumbURL     string              `json:"thumb_url,omitempty"`
	InputMessage InputMessageContent `json:"input_message_content"`
}

func (r InlineQueryResultArticle) GetType() string {
	return r.Type
}

type InputMessageContent struct {
	MessageText string `json:"message_text"`
	ParseMode   string `json:"parse_mode,omitempty"`
}

type AnswerInlineQuery struct {
	InlineQueryID string              `json:"inline_query_id"`
	Results       []InlineQueryResult `json:"results"`
	CacheTime     int                 `json:"cache_time,omitempty"`
	IsPersonal    bool                `json:"is_personal,omitempty"`
}
