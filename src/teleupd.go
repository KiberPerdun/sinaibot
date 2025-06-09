package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

/* ---------- события ---------- */

type MessageEvent struct {
	UserID int64
	Text   string
}

type InlineEvent struct {
	QueryID string
	UserID  int64
	Query   string
}

type NewMemberEvent struct {
	UserID int64
	ChatID int64
}

/* ---------- утилита для поиска новичков ---------- */

type getChatResponse struct {
	Ok     bool `json:"ok"`
	Result struct {
		ID        int64  `json:"id"`
		FirstName string `json:"first_name"`
		UserName  string `json:"username"`
	} `json:"result"`
}

/* ------------- модели (api_models.go, или где у вас хранится) --------- */

type telegramUpdate struct {
	UpdateID    int64             `json:"update_id"`
	Message     *telegramMessage  `json:"message,omitempty"`
	InlineQuery *inlineQuery      `json:"inline_query,omitempty"`
	ChatMember  *chatMemberUpdate `json:"chat_member,omitempty"` // ← NEW
}

/* -- сервис-сообщение о смене статуса участника ------------------------ */

type chatMemberUpdate struct {
	Chat          telegramChat     `json:"chat"`
	From          telegramUser     `json:"from"`
	Date          int64            `json:"date"`
	OldChatMember chatMemberStatus `json:"old_chat_member"`
	NewChatMember chatMemberStatus `json:"new_chat_member"`
}

type chatMemberStatus struct {
	User   telegramUser `json:"user"`
	Status string       `json:"status"` // "member", "left", "kicked", …
}

/* -- обычное сообщение, но с массивом NewChatMembers ------------------- */

type telegramMessage struct {
	MessageID int64        `json:"message_id"`
	From      telegramUser `json:"from"`
	Chat      telegramChat `json:"chat"`
	Date      int64        `json:"date"`
	Text      string       `json:"text"`

	NewChatMembers []telegramUser `json:"new_chat_members,omitempty"` // ← NEW
}

/* ---------------------------------------------------------------------- */

// DetectNewMemberUIDs вытаскивает UID-ы всех новых участников из update.
// Работает и для service-сообщений new_chat_members, и для chat_member.
func DetectNewMembers(u telegramUpdate) []NewMemberEvent {
	var evts []NewMemberEvent

	// 1) chat_member
	if cm := u.ChatMember; cm != nil &&
		cm.NewChatMember.Status == "member" &&
		cm.OldChatMember.Status != "member" {

		evts = append(evts, NewMemberEvent{
			UserID: cm.NewChatMember.User.ID,
			ChatID: cm.Chat.ID,
		})
	}

	// 2) service-message new_chat_members
	if m := u.Message; m != nil && len(m.NewChatMembers) > 0 {
		for _, usr := range m.NewChatMembers {
			evts = append(evts, NewMemberEvent{
				UserID: usr.ID,
				ChatID: m.Chat.ID,
			})
		}
	}

	return evts
}

/* ---------- запуск опроса ---------- */

// startUpdatePolling long-poll’ит Telegram и рассылает:
//   - текстовые сообщения  → msgCh
//   - inline-запросы       → inlineCh
//   - UID новичков         → newMemCh
//
// Закройте stopCh для корректного завершения.
func startUpdatePolling(c *tgClient) (
	msgCh chan MessageEvent,
	inlineCh chan InlineEvent,
	newMemCh chan NewMemberEvent,
	stopCh chan struct{},
) {
	msgCh = make(chan MessageEvent, 100)
	inlineCh = make(chan InlineEvent, 100)
	newMemCh = make(chan NewMemberEvent, 100)
	stopCh = make(chan struct{})

	go func() {
		offset := int64(0)
		backoff := time.Second

		for {
			select {
			case <-stopCh:
				close(msgCh)
				close(inlineCh)
				close(newMemCh)
				return
			default:
			}

			updates, err := getUpdates(c, offset)
			if err != nil {
				logerr(err, "getupdates error", "startUpdatePolling")
				time.Sleep(backoff)
				if backoff < 30*time.Second {
					backoff *= 2
				}
				continue
			}
			backoff = time.Second

			for _, u := range updates {
				// можно залогировать при необходимости
				_, _ = json.MarshalIndent(u, "", "  ")

				// --- текстовые сообщения
				if u.Message != nil && u.Message.Text != "" {
					msgCh <- MessageEvent{
						UserID: u.Message.Chat.ID,
						Text:   u.Message.Text,
					}
					chatid_to_user[u.Message.Chat.ID] = u.Message.From
				}

				// --- inline-запросы
				if iq := u.InlineQuery; iq != nil {
					inlineCh <- InlineEvent{
						QueryID: iq.ID,
						UserID:  iq.From.ID,
						Query:   iq.Query,
					}
				}

				// --- новые участники
				ss, _ := json.Marshal(u)
				fmt.Println(string(ss))
				ResNewMem := DetectNewMembers(u)
				for i := 0; i < len(ResNewMem); i++ {
					newMemCh <- NewMemberEvent{UserID: ResNewMem[i].UserID, ChatID: ResNewMem[i].ChatID}
				}

				if u.UpdateID >= offset {
					offset = u.UpdateID + 1
				}
			}
		}
	}()

	return
}

func GetUserNamesByID(userID int64) (firstName, userName string, err error) {
	token, _ := loadTokens(keysFile)
	// Формируем URL: https://api.telegram.org/bot<token>/getChat?chat_id=<userID>
	url := fmt.Sprintf("https://api.telegram.org/bot%s/getChat?chat_id=%d", token, userID)

	// Делаем HTTP GET
	resp, err := http.Get(url)
	if err != nil {
		return "", "", fmt.Errorf("ошибка HTTP GET: %w", err)
	}
	defer resp.Body.Close()

	// Декодируем JSON
	var data getChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", "", fmt.Errorf("не удалось распарсить JSON: %w", err)
	}

	// Проверяем, что Telegram вернул ok = true
	if !data.Ok {
		return "", "", fmt.Errorf("Telegram API вернул ok = false")
	}

	// Возвращаем first_name и username (username может быть пустым, если у юзера нет @username)
	return data.Result.FirstName, data.Result.UserName, nil
}
