package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

const (
	// Токен для боевого бота
	production_token = ""
	// Токен для алерт-бота
	alert_token = ""
	// Базовый URL Telegram API
	api_base_url = "https://api.telegram.org/bot"
	// Метод для получения апдейтов
	get_updates_method = "/getUpdates"
	// URL для получения апдейтов
	api_url_get = api_base_url + production_token + get_updates_method
	// URL для отправки сообщений
	api_url_send = api_base_url + production_token
	// URL для отправки сообщений алерт-ботом
	api_url_send_alert = api_base_url + alert_token
)

var inner_mutex_1 = &sync.Mutex{}
var inner_uid int64
var inner_txtc string

// Структура простого сообщения без кнопок
type outgoing_message_simple struct {
	chat_id         int    `json:"chat_id"`                  // ID чата, куда отправлять сообщение
	message_text    string `json:"text"`                     // Текст сообщения
	parse_mode      string `json:"parse_mode"`               // Режим парсинга (Markdown)
	disable_preview bool   `json:"disable_web_page_preview"` // Отключить превью ссылок
}

// Информация о чате пользователя
type user_chat struct {
	chat_id   int    `json:"id"`       // ID чата
	user_name string `json:"username"` // Имя пользователя
}

// Структура входящего сообщения
type incoming_message struct {
	chat         user_chat `json:"chat"` // Чат
	message_text string    `json:"text"` // Текст
}

// Кнопка клавиатуры
type button struct {
	text string `json:"text"` // Текст кнопки
}

// Структура сообщения с кнопками
type outgoing_message struct {
	chat_id         int           `json:"chat_id"`                  // ID чата
	message_text    string        `json:"text"`                     // Текст сообщения
	keyboard_markup keyboard_data `json:"reply_markup"`             // Клавиатура
	parse_mode      string        `json:"parse_mode"`               // Режим парсинга (Markdown)
	disable_preview bool          `json:"disable_web_page_preview"` // Отключить превью ссылок
}

// Данные для клавиатуры
type keyboard_data struct {
	keyboard        [][]button `json:"keyboard"`        // Клавиатура
	resize_keyboard bool       `json:"resize_keyboard"` // Автоматический размер
	is_persistent   bool       `json:"is_persistent"`   // Клавиатура постоянная
}

// Функция отправки сообщения с кнопками
func send_message_with_buttons(message_text string, chat_id int, button_labels []string, is_persistent bool, buttons_per_row, break_row_after int) {
	total_buttons := uint8(len(button_labels)) // Общее количество кнопок
	var message outgoing_message
	var current_row []button
	// Формируем ряды кнопок
	for i := 1; uint8(i) <= total_buttons; i++ {
		current_button := button{text: button_labels[i-1]}
		current_row = append(current_row, current_button)
		// Если ряд заполнен или дошли до спец. ряда
		if len(current_row) == buttons_per_row || i == break_row_after {
			message.keyboard_markup.keyboard = append(message.keyboard_markup.keyboard, current_row)
			current_row = []button{}
		}
	}
	// Добавляем оставшиеся кнопки
	if len(current_row) > 0 {
		message.keyboard_markup.keyboard = append(message.keyboard_markup.keyboard, current_row)
	}
	// Настройки клавиатуры
	message.keyboard_markup.resize_keyboard = true
	message.keyboard_markup.is_persistent = is_persistent
	message.disable_preview = true
	message.parse_mode = "Markdown"
	message.chat_id = chat_id
	message.message_text = message_text
	// Кодируем сообщение в JSON
	data, err := json.Marshal(message)
	if err != nil {
		fmt.Println("Ошибка кодирования JSON:", err.Error())
		return
	}
	// Отправляем POST-запрос
	_, _ = http.Post(api_url_send+"/sendMessage", "application/json", bytes.NewBuffer(data))
}

// Функция отправки простого текста без кнопок
func send_simple_message(message_text string, chat_id int) {
	var message outgoing_message_simple
	message.chat_id = chat_id
	message.message_text = message_text
	message.parse_mode = "Markdown"
	message.disable_preview = true
	// Кодируем в JSON
	data, err := json.Marshal(message)
	if err != nil {
		panic("Ошибка кодирования JSON: " + err.Error())
	}
	// Отправляем POST-запрос
	_, _ = http.Post(api_url_send+"/sendMessage", "application/json", bytes.NewBuffer(data))
}
