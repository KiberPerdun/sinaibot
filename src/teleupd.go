package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

type telegram_update_response struct {
	Ok     bool              `json:"ok"`
	Result []telegram_update `json:"result"`
}
type telegram_update struct {
	UpdateId int64            `json:"update_id"`
	Message  telegram_message `json:"message"`
}
type telegram_message struct {
	MessageId int64         `json:"message_id"`
	From      telegram_user `json:"from"`
	Chat      telegram_chat `json:"chat"`
	Date      int64         `json:"date"`
	Text      string        `json:"text"`
}
type telegram_user struct {
	Id           int64  `json:"id"`
	IsBot        bool   `json:"is_bot"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	Username     string `json:"username"`
	LanguageCode string `json:"language_code"`
	IsPremium    bool   `json:"is_premium"`
}
type telegram_chat struct {
	Id        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
	Type      string `json:"type"`
}

func update(offset int64) []telegram_update {
	url := api_url_get + fmt.Sprintf("?offset=%d", offset)
again:
	resp, err := http.Get(url)
	if err != nil {
		goto again
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		goto again
	}
	var Response telegram_update_response
	fmt.Println(string(body))
	err = json.Unmarshal(body, &Response)
	if err != nil {
		goto again
	}

	return Response.Result
}
func Recv() {
	offset := int64(0)
	info := update(offset)
	offset = info[len(info)-1].UpdateId
	for {
		info := update(offset)
		if len(info) > 0 {
			/*go func() {
				STRUCTURES.UserNamesByIdMu.Lock()
				STRUCTURES.UsernamesById[info[len(info)-1].Message.Chat.ChatId] = info[len(info)-1].Message.Chat.Username
				STRUCTURES.UserNamesByIdMu.Unlock()
			}() NOT NEEDED RN*/
			offset = (info[len(info)-1].UpdateId) + 1
			text := info[len(info)-1].Message.Text
			userId := info[len(info)-1].Message.Chat.Id
			inner_mutex_1.Lock()
			inner_uid = userId
			inner_txtc = text
			inner_mutex_1.Unlock()
		}
	}
}
