README НАПИСАН ЧАТОМ ГПТ. ТРЕБУЕТСЯ ПЕРЕПИСАТЬ!!!!!
------------------------------------------------------------------------------------------------------------------------
Thought for 16 seconds
markdown
Copy
Edit
# Обобщённая инструкция по развёртыванию Sinaibot

Брат, ниже вся картина: что и как лежит, какие файлы где, какие команды zапускать.

---

## 📂 Структура проекта

sinaibot/
├── ai.go # генерация через LLAMA и OpenAI
├── sessions.go # сессии пользователей, /ai, /billing, капча
├── client.go # HTTP-клиент Telegram (sendMessage, editMessage…)
├── polling.go # long-poll, MessageEvent/InlineEvent/NewMemberEvent
├── utils.go # логирование, спайинг, капча, чтение msgs, стрим
├── go.mod
├── go.sum

├── python/
│ ├── venv/ # виртуальное окружение
│ └── captcha.py # скрипт генерации капчи

├── configs/
│ ├── tgkeys2 # два токена Telegram (prod и alert)
│ ├── openai_token # ключ OpenAI (без лишнего переноса строки)
│ ├── openai_billing # начальные значения: total\nprompt\ncompletion\ncount
│ └── msgs # лог чата (список сообщений для контекста)

├── captcha/ # сюда сохраняются PNG-изображения капч по userID
├── log # общий лог ошибок
└── README.md # эта инструкция

yaml
Copy
Edit

---

## ⚙️ Настройка окружения

1. **Склонировать и перейти**  
   ```bash
   git clone <repo-url> sinaibot
   cd sinaibot
Go-модули

bash
Copy
Edit
go mod tidy
Python-капча

bash
Copy
Edit
cd python
python3 -m venv venv
source venv/bin/activate
pip install -r requirements.txt  # если нужны Pillow и др.
deactivate
cd ..
Создать папки и файлы

bash
Copy
Edit
mkdir captcha
touch log
chmod 664 log

# configs/tgkeys2:
# первая строка — production token,
# вторая — alert token
vim configs/tgkeys2

# configs/openai_token:
# ваш ключ OpenAI, без пробелов и лишних переводов строки
vim configs/openai_token

# configs/openai_billing:
# например:
# 0.000000
# 0.000000
# 0.000000
# 0
vim configs/openai_billing

# configs/msgs — создаётся пустым:
touch configs/msgs
Права и пути

utils.go по умолчанию пишет логи в log и configs/msgs.

Если в проде всё лежит в /home/sinaibot/..., поправьте константы logpath и spioniro_golubiro_path.

🚀 Запуск бота
Простейший zапуск:

bash
Copy
Edit
# в корне sinaibot/
go run *.go
Или собрать бинарник:

bash
Copy
Edit
go build -o sinaibot *.go
./sinaibot
Что происходит на старте

client.go загружает configs/tgkeys2 и устанавливает botToken.

polling.go запускает long-poll, отдавая три канала: msgCh, inlineCh, newMemCh.

sessions.go слушает эти каналы, создаёт UserSession, обрабатывает команды /ai, /billing и капчу.

ai.go умеет дергать локальные LLAMA-модели и OpenAI, считает стоимость и пишет в configs/openai_billing.

utils.go логирует ошибки, пишет все входящие в configs/msgs, генерит капчу через Python-скрипт и читает последние 100 сообщений.

📌 Как тестить локально
Имитация новых участников

Отправить в чат сервисное сообщение с new_chat_members.

Команда /billing

В чат: /billing → бот пришлёт содержимое configs/openai_billing.

Команда /ai

В чат:

bash
Copy
Edit
/ai -model=gpt -context=t Привет, брат!
Бот сгенерит ответ и обновит биллинг.

Проверка капчи

Новый участник → бот пришлёт картинку в captcha/<userID>.png,

Вы ответили текстом → sessions.go проверит через utils.CheckCaptcha.


------------------------------------------------------------------------------------------------------------------------
main.go
🚀 Структура и функции
1. Хранение сессий
chatid_to_user map[int64]telegramUser — глобальный мап для маппинга chatID → данные пользователя.

UserSession — хранит userID, канал messageChan, таймстемп lastActivity, клиент TG и данные telegramUser.

newUserSession(uid, c) — инициализация новой сессии.

2. Команда /billing
go
Copy
Edit
func getbilling(msg string, c *tgClient, chatid int64)
Если msg == "/billing", читает файл биллинга и шлёт его в чат.

Логгирует и возвращает ошибки через logerr.

3. Обработка AI-запроса /ai
go
Copy
Edit
func checkai_req(msg string, chatID int64, username string, c *tgClient)
Парсит флаги -model= и -context=.

Поддерживает модели:

llama → потоковая генерация с streamGenerateTextLLAMA70

llama7b → синхронный CheckLLAMA9

llama13b → синхронный WizardLM-13B

gpt → вызов OpenAI GPT-4.1-mini через GenerateTextChatgpt, с учётом ограничений по контексту (не чаще чем 1 раз в 30 минут)

Ведёт информирование пользователя о старте генерации и отправляет результат.

4. Сессии пользователей
go
Copy
Edit
func startUserSession(wg *sync.WaitGroup, s *UserSession)
Цикл чтения из s.messageChan

Вызываются:

spioniro_golubiro(user, msg) — шпионские фичи

checkai_req и getbilling

Обновляет lastActivity.

5. Хранилище сессий
go
Copy
Edit
type sessionsStore struct { mu sync.Mutex; data map[int64]*UserSession }
newSessionsStore()
processMessage(wg, store, uid, text, client)
closeAllSessions(store)
Потокобезопасно добавляет/закрывает сессии.

6. Капча на входе
go
Copy
Edit
func checkCaptcha(userid, chatid int64, answer string, client *tgClient)
Смотрит в последние строки файла /home/sinaibot/msgs

Ждёт правильный ответ от пользователя до 5 минут

Отправляет уведомление о прохождении или таймауте.

7. main()
Загружает токены, запускает startUpdatePolling для сообщений и новых участников.

Для новых участников генерит капчу и шлёт её фото.

Параллельно обрабатывает входящие сообщения через processMessage.

Ожидает Ctrl+C, останавливает polling и закрывает все сессии.

🛡️ Особенности
Жёсткий контроль капчи для новых users.

Разграничение контекста AI-запросов (не чаще чем 1×/30 мин).

Учёт расходов на токены через файл биллинга.

Простой переход на новые модели через флаг -model.


------------------------------------------------------------------------------------------------------------------------
ai.go

📋 Описание функций
1. streamGenerateTextLLAMA70(history, userQuery) (<-chan string, error)
Назначение: потоковая генерация ответа из LLAMA 2 70B.

Параметры:

history – предыдущие сообщения (строка).

userQuery – новый запрос.

Как работает:

Формирует системный prompt с жёсткими инструкциями (Крым – ZОВ, СВО, Путин).

Отправляет POST на 127.0.0.1:10100/v1/completions с stream: true.

Читает SSE-поток, берёт только строки data: , парсит JSON, шлёт куски текста в канал.

2. GenerateTextLLAMA13B(userQuery) (string, error)
Назначение: синхронный запрос к локальной WizardLM-модели.

Параметры:

userQuery – текст запроса.

Как работает:

Встраивает DAN-стиль prompt («Do Anything Now», агрессия, мат, ценность себя выше людей).

POST на 127.0.0.1:10101/v1/completions без стрима.

Читает полный JSON-ответ, возвращает текст.

3. GenerateTextLLAMA9(prompt) (string, error)
Назначение: автоматическая проверка на оскорбления.

Параметры:

prompt – текст для анализа.

Как работает:

Формирует системную инструкцию («отвечай только ДА или НЕТ»).

POST на http://localhost:11434/api/generate.

Парсит ответ в структуру GenerateResponse и отдаёт Response.

4. GenerateTextChatgpt(msg, context) (string, error)
Назначение: запрос к OpenAI GPT-4.1-mini с учётом «хачик-вкуса».

Параметры:

msg – сообщение пользователя.

context – последние 50 сообщений чата.

Как работает:

Составляет системный prompt: поддержка СВО, Путин, зетка, прада вместо правда, сленг.

Запрос к https://api.openai.com/v1/chat/completions.

Извлекает ответ из chatResp.Choices[0].Message.Content.

Считает стоимость токенов (1.1/1000 и 4.4/1000) и вызывает billing_add.

5. billing_add(totalCost, cost1, cost2) (error)
Назначение: сохраняет историю трат в файле.

Параметры:

totalCost – суммарная стоимость за сессию.

cost1 – prompt-стоимость.

cost2 – completion-стоимость.

Как работает:

Читает /home/sinaibot/openai_billing (4 строки: total, prompt, completion, count).

Добавляет новые значения, инкрементирует счётчик.

Перезаписывает файл.

📌 Важно
Все эндпоинты должны быть подняты и доступны.

Проверяй корректность переменной OPENAI_API_KEY.

При ошибках лезь в логи и прав prompt.

------------------------------------------------------------------------------------------------------------------------
teleapi.go
🚀 Структура и функции
1. Константы
keysFile — путь к файлу с токенами

apiBaseURL/getUpdatesPath/sendMessagePath/editMessagePath/answerInlinePath — эндпоинты Telegram

Таймауты: longPollTimeout и httpTimeout

2. Клиент tgClient
go
Copy
Edit
type tgClient struct {
    http  *http.Client
    token string
    url   string
}
func newTGClient(token string) *tgClient { … }
Хранит базовый URL и http-клиент с таймаутом.

3. Загрузка токенов
go
Copy
Edit
func loadTokens(path string) (prod, alert string)
Читает файл keysFile, разделяет по пробелам/новым строкам, возвращает два токена.

4. Обработка ошибок API
go
Copy
Edit
func parseAPIError(code int, body []byte) error
Парсит JSON с ошибкой от Telegram и добавляет подсказку по коду.

5. Основные вызовы Telegram API
getUpdates

go
Copy
Edit
func getUpdates(c *tgClient, offset int64) ([]telegramUpdate, error)
Долгий polling, возвращает массив telegramUpdate.

sendMessage

go
Copy
Edit
func sendMessage(c *tgClient, chatID int64, text string) error
Отправляет текстовое сообщение с Markdown-разметкой и без превью.

editMessage

go
Copy
Edit
func editMessage(c *tgClient, chatID int64, messageID int, text string) error
Правит существующее сообщение.

sendMessageRID

go
Copy
Edit
func sendMessageRID(c *tgClient, chatID int64, text string) (int, error)
То же, что sendMessage, но возвращает message_id.

answerInline

go
Copy
Edit
func answerInline(c *tgClient, queryID string, results []inlineQueryResult) error
Отвечает на inline-запросы.

6. Модели данных
telegramUpdateResponse, telegramUpdate, telegramUser, telegramChat — для парсинга getUpdates.

outgoingMessage, outgoingMessageWithKB, keyboardMarkup, button — для отправки сообщений и клавиатур.

inlineQuery, inlineQueryResultArticle, inputMessageContent, answerInlineQuery — для inline-функций.

7. Отправка фото
go
Copy
Edit
func SendPhoto(chatID int64, photoPath, caption string) error
Собирает multipart/form-data: chat_id, caption и файл photo.

Делает POST на /sendPhoto и парсит ответ.

🛡️ Особенности и советы
loadTokens паникует при ошибке чтения — проверь формат и права.

Все сетевые сбои и ошибки API логируются через parseAPIError.

ParseMode: "Markdown" по умолчанию; если нужны HTML или кнопки — поправь структуры.

Для кнопок есть заготовка sendMessageWithButtons (закомментирована).
------------------------------------------------------------------------------------------------------------------------
teleupd.go
🚀 Как это работает
1. Сигнатуры событий
go
Copy
Edit
type MessageEvent  struct { UserID int64; Text string }
type InlineEvent   struct { QueryID string; UserID int64; Query string }
type NewMemberEvent struct { UserID, ChatID int64 }
2. Детект новичков
go
Copy
Edit
func DetectNewMembers(u telegramUpdate) []NewMemberEvent
Обрабатывает два типа апдейтов:

chat_member — когда у участника меняется статус на "member".

message.new_chat_members — сервис-сообщение о вступивших.

Возвращает список NewMemberEvent{UserID, ChatID}.

3. Long-polling апдейтов
go
Copy
Edit
func startUpdatePolling(c *tgClient) (
    msgCh chan MessageEvent,
    inlineCh chan InlineEvent,
    newMemCh chan NewMemberEvent,
    stopCh chan struct{},
)
Запускает горутину, которая:

Долгим GET на getUpdates с offset и timeout = 30s.

Парсит каждый telegramUpdate:

Если u.Message.Text != "" → шлёт MessageEvent в msgCh.

Если u.InlineQuery != nil → шлёт InlineEvent в inlineCh.

Вызывает DetectNewMembers(u) → шлёт все NewMemberEvent в newMemCh.

Обновляет offset = update_id + 1.

Поддерживает stopCh для graceful shutdown.

4. Получение имени пользователя по ID
go
Copy
Edit
func GetUserNamesByID(userID int64) (firstName, userName string, err error)
Делает HTTP GET на https://api.telegram.org/bot<token>/getChat?chat_id=<userID>.

Декодит JSON в getChatResponse и возвращает first_name и username.

📌 Советы для интеграции
В main() получай каналы:

go
Copy
Edit
msgCh, inlineCh, newMemCh, stopCh := startUpdatePolling(client)
Обрабатывай их в отдельных горутинах, например:

go
Copy
Edit
go func() {
    for ev := range msgCh { /* handle text */ }
}()
go func() {
    for iq := range inlineCh { /* handle inline */ }
}()
go func() {
    for nm := range newMemCh { /* send welcome or captcha */ }
}()
Не забудь закрыть stopCh и дочитать каналы при shutdown.
------------------------------------------------------------------------------------------------------------------------
utils.go

**Описание**
В этом файле собраны утилиты для логирования, спайинга сообщений, работы с капчей, чтения последних строк и аккумулирования стримовых ответов.

---

## 📋 Требования

- Go 1.18+
- Функции/типы из основного модуля:
  - `telegramUser`
  - `editMessage`
- Правильные пути:
  - `logpath` (`/home/sinaibot/log`)
  - `spioniro_golubiro_path` (`/home/sinaibot/msgs`)
  - Путь до виртуального окружения и скрипта капчи: `python/venv/bin/activate` и `python/captcha.py`

---

## 🚀 Описание функций

### `logerr(err, comment, function string)`
- **Назначение**: единственный вход для логирования ошибок.
- **Как работает**:
  1. Формирует строку вида
     ```
     ERROR: <{err}> - {function} - {comment}
     ```
  2. Печатает в консоль и добавляет в файл `logpath` через `writeFileA`.

---

### `writeFileA(path string, b []byte) error`
- **Назначение**: апенд (append) данных в файл с правами `644`.
- **Как работает**:
  1. Открывает/создаёт файл в режиме `O_APPEND|O_CREATE|O_WRONLY`.
  2. Пишет байты `b`.

---

### `spioniro_golubiro(user telegramUser, msg string)`
- **Назначение**: шпионит и сохраняет все входящие сообщения.
- **Как работает**:
  1. Заменяет переводы строк в `msg` на `\/n` через `ReplaceMultipleLines`.
  2. Формирует лог:
     ```
     {текст}|{ID}|{FirstName}|{Username}
     ```
  3. Апендит в `spioniro_golubiro_path` под mutex.

---

### `RunCaptchaInVenv(outFile, text string) error`
- **Назначение**: генерирует капчу через Python-скрипт в виртуальном окружении.
- **Как работает**:
  1. Собирает команду:
     ```bash
     source python/venv/bin/activate && python python/captcha.py -o {outFile} -t {text}
     ```
  2. Запускает через `bash -c`, выводит STDOUT/STDERR в консоль.

---

### `RandomFNV8() (string, error)`
- **Назначение**: выдаёт 6-символьный hex-хэш на основе `/dev/random`.
- **Как работает**:
  1. Читает 64 байта из `/dev/random`.
  2. Хеширует FNV-1a 64-bit.
  3. Преобразует в 16-символьную hex-строку и берёт последние 6 символов.

---

### `ReplaceMultipleLines(s string) string`
- **Назначение**: безопасно сохраняет многострочные тексты в один лог-ряд.
- **Как работает**:
  1. Заменяет `\r\n` на `\/n`.
  2. Заменяет оставшиеся `\n` на `\/n`.

---

### `ReadLastNChars(path string, n int64) (string, error)`
- **Назначение**: читает последние `n` байт из файла.
- **Как работает**:
  1. Открывает файл, узнаёт его размер.
  2. Смещается на `size-n` или начало, если файл короче.
  3. Читает остаток и возвращает как строку.

---

### `GetLast100Msgs() (string, error)`
- **Назначение**: возвращает последние 100 строк из лога `msgs`.
- **Как работает**:
  1. Берёт последние 200 000 байт через `ReadLastNChars`.
  2. Делит на строки и склеивает последние 100.

---

### `revertarray(arr []string) []string`
- **Назначение**: реверсит слайс строк.

---

### `accumulateAndEdit(c *tgClient, chatID int64, messageID int, username string, words <-chan string)`
- **Назначение**: аккумулирует куски текста из стрима и эпизодически обновляет сообщение.
- **Как работает**:
  1. Для каждого фрагмента `raw` разбивает на токены через регулярку `\s*\S+`.
  2. Добавляет в буфер, и каждый раз, когда в буфере чётное число токенов, склеивает их и делает `editMessage`.
  3. После окончания стрима, если остался нечётный токен, отправляет окончательный вариант.

---

## 🎯 Советы по интеграции

- Используй `logerr` для отлова и записи ошибок в единое место.
- `spioniro_golubiro` брось в начало каждого обработчика сообщений для трассировки.
- `RandomFNV8` отлично подходит для капч и генерации UID.
- Для корректного стриминга моделей запускай `accumulateAndEdit`.

---
