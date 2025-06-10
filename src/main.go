package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"
)

/* ---------- session ---------------------------------------------------- */

var chatid_to_user map[int64]telegramUser
var auth_users = []string{"programju", "KiberPerdun", "nakiperu", "nullpointerrr", "potsield", "jotunn_polaris", "wh1t34ox", "0xEEEEE", "xfrbt", "Softgod", "bluprod", "Ciscouse", "go_B_tanki", "dogdjgift", "Tunay69", "egunuraka", "cq_rs", "Shonoy", "vehsorg", "marina_cpp", "press_to_pnick", "unknownguy228", "vietnam_veteran", "нуль форма", "heaven99990", "oldestme", "konakona06", "nextdoor_psycho", "umchg", "neutraluser", "kurumihere", "pablusha", "qat_ears", "na1tero", "lomosgame228", "OneBumBot", "jackpot_enjoyer", "rafchapw", "kiqoi", "gj0dfrg39f7v7eanpf90e0re8i19h5ku", "unsignedlong", "q1w23re", "refrct", "kotvkvante", "kellaux", "I0xbkaker", "c4llv07e", "A11131111", "val_ep", "char_ptr", "nfjdkq", "nagornin", "ivanvet31"}

type UserSession struct {
	userID       int64
	messageChan  chan string
	lastActivity time.Time
	client       *tgClient
	User         telegramUser
}

func newUserSession(uid int64, c *tgClient) *UserSession {
	return &UserSession{
		userID:       uid,
		messageChan:  make(chan string, 10),
		lastActivity: time.Now(),
		client:       c,
	}
}

func getbilling(msg string, c *tgClient, chatid int64) {
	if msg != "/billing" {
		return
	}

	f, err := os.ReadFile("/home/sinaibot/openai_billing")
	if err != nil {
		logerr(err, "error reading file", "getbilling")
		return
	}

	sendMessage(c, chatid, string(f))
}

var lastcontextusage int64

func checkai_req(msg string, chatID int64, username string, c *tgClient) {
	if strings.Split(msg, " ")[0] != "/ai" {
		return
	}
	// разбиваем команду на поля
	parts := strings.Fields(msg)
	// нужно минимум: /ai model=... -context=... вопрос
	if len(parts) < 4 || parts[0] != "/ai" {
		_ = sendMessage(c, chatID,
			fmt.Sprintf("%s, неверный формат. Используй: /ai -model=... -context=... вопрос", username))
		return
	}

	var model, context, question string
	idx := 1
	// проходим по флагам
	for ; idx < len(parts); idx++ {
		token := parts[idx]
		if strings.HasPrefix(token, "-model=") {
			model = strings.TrimPrefix(token, "-model=")
		} else if strings.HasPrefix(token, "-context=") {
			context = strings.TrimPrefix(token, "-context=")
		} else {
			break
		}
	}
	// остальное — это сам запрос
	question = strings.Join(parts[idx:], " ")

	// проверяем, что всё есть
	if model == "" || context == "" || question == "" {
		_ = sendMessage(c, chatID,
			fmt.Sprintf("%s, неверный формат. Пример: /ai model=gpt -context=твой_контекст вопрос", username))
		return
	}

	// теперь model, context и question заданы — можно использовать дальше
	switch model {
	case "llama":
		{
			if context != "f" && context != "t" {
				_ = sendMessage(c, chatID, "Контекст может быть только t или f")
				return
			}

			if context != "f" {
				_ = sendMessage(c, chatID, "Модель не поддерживает контекст")
			}

			var err error
			msgID, err := sendMessageRID(c, chatID,
				fmt.Sprintf("генерация (model=%s, context=%s) началась...", model, context))
			if err != nil {
				logerr(err, "sendMessage", "checkai_req")
				return
			}

			// передаём модель и вопрос в стрим-генерацию (добавь context в сам вызов, если поддерживается)
			wordsChan, err := streamGenerateTextLLAMA70(model, question)
			if err != nil {
				logerr(err, "streamGenerate", "checkai_req")
				return
			}

			go accumulateAndEdit(c, chatID, msgID, username, wordsChan)
		}
	case "llama7b":
		{
			if context != "f" && context != "t" {
				_ = sendMessage(c, chatID, "Контекст может быть только t или f")
				return
			}

			if context != "f" {
				_ = sendMessage(c, chatID, "Модель не поддерживает контекст")
			}

			// передаём модель и вопрос в стрим-генерацию (добавь context в сам вызов, если поддерживается)
			answer, err := GenerateTextLLAMA9(question)
			if err != nil {
				logerr(err, "streamGenerate", "checkai_req")
				return
			}

			sendMessage(c, chatID, answer)
		}
	case "llama13b":
		{
			if context != "f" && context != "t" {
				_ = sendMessage(c, chatID, "Контекст может быть только t или f")
				return
			}

			if context != "f" {
				_ = sendMessage(c, chatID, "Модель не поддерживает контекст")
			}

			// передаём модель и вопрос в стрим-генерацию (добавь context в сам вызов, если поддерживается)
			answer, err := GenerateTextLLAMA13B(question)
			if err != nil {
				logerr(err, "streamGenerate", "checkai_req")
				return
			}

			sendMessage(c, chatID, answer)
		}
	case "gpt":
		{
			var last100 string
			var err error

			if context != "f" && context != "t" {
				_ = sendMessage(c, chatID, "Контекст может быть только t или f")
				return
			}

			if context == "t" {
				if lastcontextusage != 0 && (time.Now().Unix()-lastcontextusage) < 60*30 {
					sendMessage(c, chatID, "Вы не можете использовать контекст чаще, чем 1 раз в 30 минут. Генерация продолжится без контекста")
					last100 = "NONE"
					goto nocontext
				}
				lastcontextusage = time.Now().Unix()
				last100, err = GetLast100Msgs()
				if err != nil {
					logerr(err, "getting last msgs", "checkai_req")
					last100 = "NONE"
				}
			} else {
				last100 = "NONE"
			}

		nocontext:

			_ = sendMessage(c, chatID, fmt.Sprintf("%s, генерация (model=%s, context=%s) началась...", username, model, context))

			answer, err := GenerateTextChatgpt(msg, last100)
			if err != nil {
				logerr(err, "error generating response", "checkai_req")
			}
			err = sendMessage(c, chatID, fmt.Sprintf("@%s, %s", username, answer))
			if err != nil {
				err = sendMessage(c, chatID, answer)
			}
		}
	default:
		{
			_ = sendMessage(c, chatID, "model not found model="+model)
		}
	}
}

var allowedchats = []int64{-1002084477597}

func startUserSession(wg *sync.WaitGroup, s *UserSession) {
	defer wg.Done()
	if !slices.Contains(allowedchats, s.userID) {
		sendMessage(s.client, s.userID, "Бот доступен только в синае")
		return
	}
	log.Printf("[session %d] started", s.userID)

	for msg := range s.messageChan {
		user := chatid_to_user[s.userID]
		go func(user telegramUser, msg string) {
			spioniro_golubiro(user, msg)
		}(user, msg)

		s.lastActivity = time.Now()

		checkai_req(msg, s.userID, user.Username, s.client)
		getbilling(msg, s.client, s.userID)
		pingall(msg, s.userID, s.client, user.Username)
		//fmt.Println(msg, s.User)
		/*if err := sendMessage(s.client, s.userID, msg); err != nil {
			diagnoseTelegramError(err)
		}*/
	}
	log.Printf("[session %d] stopped", s.userID)
}

var lastping int64

func pingall(msg string, chatid int64, c *tgClient, username string) {
	if msg != "/pingall" || !slices.Contains(auth_users, username) || (lastping != 0 && time.Now().Unix()-lastping < 60*60) {
		if (lastping != 0 && time.Now().Unix()-lastping < 60*60) && msg == "/pingall" {
			sendMessage(c, chatid, "/pingall доступна только раз в час")
		}
		return
	}

	lastping = time.Now().Unix()

	users := getallusers()
	for i := 0; i < len(users); i++ {
		users[i] = "@" + users[i]
	}
	sendMessage(c, chatid, strings.Join(users, ", "))
}

func diagnoseTelegramError(err error) {
	e := strings.ToLower(err.Error())
	switch {
	case strings.Contains(e, "bad request") && strings.Contains(e, "markdown"):
		log.Println("Совет: проверьте Markdown или уберите ParseMode")
	case strings.Contains(e, "forbidden"):
		log.Println("Совет: пользователь заблокировал бота или удалил чат")
	case strings.Contains(e, "too many requests"):
		log.Println("Совет: превысил лимит Telegram API — замедлитесь")
	}
}

/* ---------- store ------------------------------------------------------ */

type sessionsStore struct {
	mu   sync.Mutex
	data map[int64]*UserSession
}

func newSessionsStore() *sessionsStore { return &sessionsStore{data: make(map[int64]*UserSession)} }

func processMessage(wg *sync.WaitGroup, st *sessionsStore, uid int64, text string, c *tgClient) {
	st.mu.Lock()
	sess, ok := st.data[uid]
	if !ok {
		sess = newUserSession(uid, c)
		st.data[uid] = sess
		wg.Add(1)
		go startUserSession(wg, sess)
	}
	st.mu.Unlock()

	sess.messageChan <- text
}

func closeAllSessions(st *sessionsStore) {
	st.mu.Lock()
	defer st.mu.Unlock()
	for id, s := range st.data {
		log.Printf("closing session %d", id)
		close(s.messageChan)
		delete(st.data, id)
	}
}

/* ---------- inline utils ----------------------------------------------- */
/*
func processInlineCommand(q string) (string, string) {
	q = strings.TrimSpace(q)

	switch {
	case strings.HasPrefix(q, "/time"):
		return "Текущее время", fmt.Sprintf("Сейчас: *%d*", time.Now().Unix())
	case strings.HasPrefix(q, "/date"):
		return "Текущая дата", fmt.Sprintf("Сегодня: *%s*", time.Now().Format("02.01.2006"))
	case strings.HasPrefix(q, "/bold "):
		return "Жирный текст", fmt.Sprintf("*%s*", strings.TrimPrefix(q, "/bold "))
	case strings.HasPrefix(q, "/italic "):
		return "Курсивный текст", fmt.Sprintf("_%s_", strings.TrimPrefix(q, "/italic "))
	case strings.HasPrefix(q, "/help"), q == "":
		return "Справка", "*Команды:* /time /date /bold /italic /help"
	default:
		return "Эхо", q
	}
}
*/
/* ---------- main ------------------------------------------------------- */

func checkCaptcha(userid int64, chatid int64, answer string, client *tgClient) {
	timestart := time.Now().Unix()
	uid_string := fmt.Sprintf("%d", userid)
	for {
		data, err := ReadLastNChars("/home/sinaibot/msgs", 2000)
		if err != nil {
			logerr(err, "error reading msg history", "checkCaptcha")
		}
		dataS := strings.Split(data, "\n")
		n := len(dataS)
		for i := 1; /* intended */ i < n; i++ {
			line := dataS[i]
			lineS := strings.Split(line, "|")
			if len(lineS) != 4 {
				continue
			}
			if lineS[1] != uid_string {
				continue
			}

			if lineS[0] == answer {
				firstname, _, err := GetUserNamesByID(userid)
				if err != nil {
					logerr(err, "error reading user names", "checkCaptcha")
					err = sendMessage(client, chatid, fmt.Sprintf("UID %s успешно прошел каптчу", uid_string))
					if err != nil {
						logerr(err, "error sending message", "checkCaptcha")
					}
				}
				err = sendMessage(client, chatid, fmt.Sprintf("%s, успешно прошел каптчу!", firstname))
				if err != nil {
					logerr(err, "error sending message", "checkCaptcha")
				}
				return
			}
		}

		if time.Now().Unix()-timestart > 60*5 {
			username, _, err := GetUserNamesByID(userid)
			if err != nil {
				logerr(err, "error reading user names", "checkCaptcha")
				err = sendMessage(client, chatid, fmt.Sprintf("@Lomasterrr!!!!! UID %s (нет возможности получить username) не прошел каптчу!", uid_string))
				if err != nil {
					logerr(err, "error sending message", "checkCaptcha")
				}
			}
			err = sendMessage(client, chatid, fmt.Sprintf("@Lomasterrr!!!!! UID %s (%s) не прошел каптчу!", userid, username))
			if err != nil {
				logerr(err, "error sending message", "checkCaptcha")
			}
			return
		}
	}
}

func main() {
	chatid_to_user = make(map[int64]telegramUser)
	prodToken, _ := loadTokens(keysFile)

	openait, err := os.ReadFile("/home/sinaibot/openai_token")
	if err != nil {
		panic(err.Error())
	}
	OpenAIapiKey = string(openait)[:len(openait)-1]
	fmt.Println(OpenAIapiKey)

	botToken = prodToken // pochinit' nado bi
	client := newTGClient(prodToken)

	msgCh /* inlineCh*/, _, newMemCh, stopPoll := startUpdatePolling(client)
	store := newSessionsStore()
	var wg sync.WaitGroup

	/* --- обработка приходящих событий --- */

	go func() {
		for n := range newMemCh {
			_, username, err := GetUserNamesByID(n.UserID)
			if err != nil {
				logerr(err, "error reading user names", "checkCaptcha")
				username = "новый пользователь"
			}
			text, err := RandomFNV8()
			if err != nil {
				logerr(err, "error generating random", "main")
				continue
			}
			path := fmt.Sprintf("/home/sinaibot/captcha/%d.png", n.UserID)
			err = RunCaptchaInVenv(path, text)
			if err != nil {
				logerr(err, "error generating captcha", "main")
				continue
			}
			err = SendPhoto(n.ChatID, path, fmt.Sprintf("%s, пройдите каптчу.\nОтвет пишите прям в чат без каких-либо лишних символов. У вас есть 5 минут", username))
			if err != nil {
				logerr(err, "error sending photo", "main")
				continue
			}
			go checkCaptcha(n.UserID, n.ChatID, text, client)
		}
	}()

	go func() {
		for m := range msgCh {
			processMessage(&wg, store, m.UserID, m.Text, client)
		}
	}()
	/*
		go func() {
			for iq := range inlineCh {
				title, body := processInlineCommand(iq.Query)

				results := []inlineQueryResult{
					inlineQueryResultArticle{
						Type:  "article",
						ID:    "result",
						Title: title,
						InputMessage: inputMessageContent{
							MessageText: body,
							ParseMode:   "Markdown",
						},
					},
				}

				if !strings.HasPrefix(strings.TrimSpace(iq.Query), "/") && iq.Query != "" {
					for i := 1; i <= 2; i++ {
						id := strconv.Itoa(i)
						echo := fmt.Sprintf("Эхо %d: %s", i, iq.Query)
						results = append(results, inlineQueryResultArticle{
							Type:  "article",
							ID:    id,
							Title: echo,
							InputMessage: inputMessageContent{
								MessageText: echo,
								ParseMode:   "Markdown",
							},
						})
					}
				}

				if err := answerInline(client, iq.QueryID, results); err != nil {
					log.Printf("answerInline error: %v", err)
				}
			}
		}()

		NOT NEEDED RN
	*/
	/* --- ожидание Ctrl-C --- */

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	log.Println("shutting down…")
	close(stopPoll) // останавливаем long-poll
	closeAllSessions(store)
	wg.Wait()
	log.Println("bye")
}
