package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

/* ---------- константы ---------- */

const (
	keysFile         = "/home/sinaibot/tgkeys2" // <-- путь к токенам
	apiBaseURL       = "https://api.telegram.org/bot"
	getUpdatesPath   = "/getUpdates"
	sendMessagePath  = "/sendMessage"
	editMessagePath  = "/editMessageText"
	answerInlinePath = "/answerInlineQuery"
	longPollTimeout  = 30
	httpTimeout      = 60 * time.Second
)

var botToken string

/* ---------- клиент ---------- */

type tgClient struct {
	http  *http.Client
	token string
	url   string
}

func newTGClient(token string) *tgClient {
	return &tgClient{
		http:  &http.Client{Timeout: httpTimeout},
		token: token,
		url:   apiBaseURL + token,
	}
}

/* ---------- утилиты ---------- */

func loadTokens(path string) (prod, alert string) {
	data, err := os.ReadFile(path)
	if err != nil {
		logerr(err, "couldnt open file", "loadTokens")
		panic("err check logs")
	}
	lines := strings.Fields(strings.TrimSpace(string(data)))
	if len(lines) < 2 {
		logerr(fmt.Errorf("-"), "invalid format", "loadTokens")
		panic("err check logs")
	}
	return lines[0], lines[1]
}

func parseAPIError(code int, body []byte) error {
	var te struct {
		OK          bool   `json:"ok"`
		ErrorCode   int    `json:"error_code"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(body, &te); err != nil {
		logerr(err, "error unmarshalling", "parseAPIError")
		return err
	}
	hint := map[int]string{
		400: " (Bad Request)", 401: " (Unauthorized — токен?)", 403: " (Forbidden)",
		404: " (Not Found)", 409: " (Conflict)", 429: " (Too Many Requests)",
	}[code]
	if code >= 500 {
		hint = " (Ошибка на стороне Telegram)"
	}
	return fmt.Errorf("Telegram API ошибка %d: %s%s", te.ErrorCode, te.Description, hint)
}

/* ---------- вызовы Telegram API ---------- */

func getUpdates(c *tgClient, offset int64) ([]telegramUpdate, error) {
	url := fmt.Sprintf("%s%s?offset=%d&timeout=%d", c.url, getUpdatesPath, offset, longPollTimeout)
	req, _ := http.NewRequest(http.MethodGet, url, nil)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("сетевой сбой: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var r telegramUpdateResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("невалидный JSON: %w", err)
	}
	if !r.OK {
		return nil, parseAPIError(resp.StatusCode, body)
	}
	return r.Result, nil
}

func sendMessage(c *tgClient, chatID int64, text string) error {
	msg := outgoingMessage{ChatID: chatID, Text: text, ParseMode: "", DisableWebPagePreview: true}
	data, _ := json.Marshal(msg)

	req, _ := http.NewRequest(http.MethodPost, c.url+sendMessagePath, bytes.NewBuffer(data))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("сетевой сбой: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return parseAPIError(resp.StatusCode, body)
	}
	var ok struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(body, &ok); err != nil || !ok.OK {
		return errors.New("HTTP 200, но ok=false")
	}
	return nil
}

func editMessage(c *tgClient, chatID int64, messageID int, text string) error {
	payload := map[string]interface{}{
		"chat_id":    chatID,
		"message_id": messageID,
		"text":       text,
		"parse_mode": "Markdown",
	}
	data, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, c.url+editMessagePath, bytes.NewBuffer(data))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("edit сетевой сбой: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return parseAPIError(resp.StatusCode, body)
	}
	return nil
}

func sendMessageRID(c *tgClient, chatID int64, text string) (int, error) {
	msg := outgoingMessage{ChatID: chatID, Text: text, ParseMode: "Markdown", DisableWebPagePreview: true}
	data, _ := json.Marshal(msg)

	req, _ := http.NewRequest(http.MethodPost, c.url+sendMessagePath, bytes.NewBuffer(data))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, fmt.Errorf("сетевой сбой: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return 0, parseAPIError(resp.StatusCode, body)
	}
	var res struct {
		OK     bool `json:"ok"`
		Result struct {
			MessageID int `json:"message_id"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &res); err != nil || !res.OK {
		return 0, errors.New("HTTP 200, но ok=false")
	}
	return res.Result.MessageID, nil
}

/* func sendMessageWithButtons(
	c *tgClient,
	chatID int64,
	text string,
	buttonLabels []string,
	isPersistent bool,
	perRow, breakAfter int,
) error {

	var kb [][]button
	row := []button{}
	for i, label := range buttonLabels {
		row = append(row, button{Text: label})
		if len(row) == perRow || i+1 == breakAfter {
			kb = append(kb, row)
			row = []button{}
		}
	}
	if len(row) > 0 {
		kb = append(kb, row)
	}

	msg := outgoingMessageWithKB{
		ChatID: chatID, Text: text, ParseMode: "Markdown", DisableWebPagePreview: true,
		ReplyMarkup: keyboardMarkup{Keyboard: kb, ResizeKeyboard: true, IsPersistent: isPersistent},
	}
	data, _ := json.Marshal(msg)

	req, _ := http.NewRequest(http.MethodPost, c.url+sendMessagePath, bytes.NewBuffer(data))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("сетевой сбой: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return parseAPIError(resp.StatusCode, body)
	}
	var ok struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(body, &ok); err != nil || !ok.OK {
		return errors.New("HTTP 200, но ok=false")
	}
	return nil
}
NOT NEEDED RN
*/

func answerInline(c *tgClient, queryID string, results []inlineQueryResult) error {
	payload := answerInlineQuery{InlineQueryID: queryID, Results: results, CacheTime: 300, IsPersonal: true}
	data, _ := json.Marshal(payload)

	req, _ := http.NewRequest(http.MethodPost, c.url+answerInlinePath, bytes.NewBuffer(data))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("сетевой сбой: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return parseAPIError(resp.StatusCode, body)
	}
	var ok struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(body, &ok); err != nil || !ok.OK {
		return errors.New("HTTP 200, но ok=false")
	}
	return nil
}

/* ---------- модели ---------- */

type telegramUpdateResponse struct {
	OK     bool             `json:"ok"`
	Result []telegramUpdate `json:"result"`
}

type telegramUser struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	Username  string `json:"username"`
}

type telegramChat struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
}

/* ---- outgoing ---- */

type outgoingMessage struct {
	ChatID                int64  `json:"chat_id"`
	Text                  string `json:"text"`
	ParseMode             string `json:"parse_mode,omitempty"`
	DisableWebPagePreview bool   `json:"disable_web_page_preview,omitempty"`
}

type outgoingMessageWithKB struct {
	ChatID                int64          `json:"chat_id"`
	Text                  string         `json:"text"`
	ParseMode             string         `json:"parse_mode,omitempty"`
	DisableWebPagePreview bool           `json:"disable_web_page_preview,omitempty"`
	ReplyMarkup           keyboardMarkup `json:"reply_markup"`
}

type keyboardMarkup struct {
	Keyboard       [][]button `json:"keyboard"`
	ResizeKeyboard bool       `json:"resize_keyboard"`
	IsPersistent   bool       `json:"is_persistent"`
}

type button struct {
	Text string `json:"text"`
}

/* ---- inline ---- */

type inlineQuery struct {
	ID    string       `json:"id"`
	From  telegramUser `json:"from"`
	Query string       `json:"query"`
}

type inlineQueryResult interface{ GetType() string }

type inlineQueryResultArticle struct {
	Type         string              `json:"type"`
	ID           string              `json:"id"`
	Title        string              `json:"title"`
	Description  string              `json:"description,omitempty"`
	InputMessage inputMessageContent `json:"input_message_content"`
}

func (r inlineQueryResultArticle) GetType() string { return r.Type }

type inputMessageContent struct {
	MessageText string `json:"message_text"`
	ParseMode   string `json:"parse_mode,omitempty"`
}

type answerInlineQuery struct {
	InlineQueryID string              `json:"inline_query_id"`
	Results       []inlineQueryResult `json:"results"`
	CacheTime     int                 `json:"cache_time,omitempty"`
	IsPersonal    bool                `json:"is_personal,omitempty"`
}

type apiResponse struct {
	Ok          bool            `json:"ok"`
	Description string          `json:"description,omitempty"`
	Result      json.RawMessage `json:"result,omitempty"`
}

// SendPhoto отправляет в чат (по chatID) фотографию из локального файла photoPath.
// caption — необязательная подпись к фото (может быть пустой строкой).
func SendPhoto(chatID int64, photoPath, caption string) error {
	// Берём токен из переменных окружения
	// Открываем файл с фоткой
	file, err := os.Open(photoPath)
	if err != nil {
		return fmt.Errorf("не удалось открыть файл %q: %w", photoPath, err)
	}
	defer file.Close()

	// Создаём буфер и multipart writer
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// 1) Поле chat_id
	if err := writer.WriteField("chat_id", fmt.Sprint(chatID)); err != nil {
		return fmt.Errorf("ошибка при добавлении поля chat_id: %w", err)
	}

	// 2) Поле caption (даже если пустая строка, Telegram пропустит)
	if caption != "" {
		if err := writer.WriteField("caption", caption); err != nil {
			return fmt.Errorf("ошибка при добавлении поля caption: %w", err)
		}
	}

	// 3) Поле photo (сам файл). Имя поля должно быть "photo"
	part, err := writer.CreateFormFile("photo", filepath.Base(photoPath))
	if err != nil {
		return fmt.Errorf("ошибка при создании form-file: %w", err)
	}

	// Копируем содержимое файла в multipart
	if _, err := io.Copy(part, file); err != nil {
		return fmt.Errorf("не удалось скопировать содержимое файла: %w", err)
	}

	// Завершаем формирование multipart
	if err := writer.Close(); err != nil {
		return fmt.Errorf("ошибка при закрытии writer: %w", err)
	}

	// Формируем URL: https://api.telegram.org/bot<token>/sendPhoto
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendPhoto", botToken)

	// Делаем POST-запрос с нашим multipart-контентом
	req, err := http.NewRequest("POST", url, &buf)
	if err != nil {
		return fmt.Errorf("ошибка при создании запроса: %w", err)
	}
	// Устанавливаем правильный заголовок
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Выполняем запрос
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("ошибка HTTP-запроса: %w", err)
	}
	defer resp.Body.Close()

	// Декодим ответ Telegram
	var apiResp apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return fmt.Errorf("не удалось распарсить ответ Telegram: %w", err)
	}

	if !apiResp.Ok {
		return fmt.Errorf("Telegram API вернул ошибку: %s", apiResp.Description)
	}

	return nil
}
