package main

import (
	"fmt"
	"hash/fnv"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
)

var logpath = "/home/sinaibot/log"
var spioniro_golubiro_mu sync.Mutex
var spioniro_golubiro_path = "/home/sinaibot/msgs"

func logerr(err error, comment, function string) {
	errmsg := fmt.Sprintf("\nERROR: <%s> - %s - %s\n", err.Error(), function, comment)
	fmt.Println(errmsg)
	writeFileA(logpath, []byte(errmsg))
}

func writeFileA(path string, b []byte) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(b)
	return err
}

func spioniro_golubiro(user telegramUser, msg string) {
	log := fmt.Sprintf("%s|%d|%s|%s\n", ReplaceMultipleLines(msg), user.ID, user.FirstName, user.Username)
	spioniro_golubiro_mu.Lock()
	err := writeFileA(spioniro_golubiro_path, []byte(log))
	if err != nil {
		logerr(err, "error writing", "spioniro_golubiro")
	}
	spioniro_golubiro_mu.Unlock()
}

func RunCaptchaInVenv(outFile, text string) error {
	// Собираем команду: source venv/bin/activate && python scriptPath -o outFile -t text
	cmdStr := fmt.Sprintf(
		"source python/venv/bin/activate && python python/captcha.py -o %s -t %s", outFile, text)
	cmd := exec.Command("bash", "-c", cmdStr)

	// Перенаправляем вывод в терминал, чтобы видеть прогресс/ошибки
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func RandomFNV8() (string, error) {
	// Открываем /dev/random
	f, err := os.Open("/dev/random")
	if err != nil {
		return "", fmt.Errorf("не удалось открыть /dev/random: %w", err)
	}
	defer f.Close()

	// Читаем 64 байта
	buf := make([]byte, 64)
	if _, err := io.ReadFull(f, buf); err != nil {
		return "", fmt.Errorf("не удалось прочитать рандом: %w", err)
	}

	// Считаем FNV-1a 64-bit
	hasher := fnv.New64a()
	_, _ = hasher.Write(buf)
	sum := hasher.Sum64()

	// Превращаем в 16-ричную строку (точно 16 символов, с нулями впереди)
	fullHex := fmt.Sprintf("%016x", sum)

	// Берём последние 8 символов
	last8 := fullHex[len(fullHex)-6:]
	return last8, nil
}

func ReplaceMultipleLines(s string) string {
	// Если нужно учитывать CRLF ("\r\n"), сперва заменим их на "\/n":
	s = strings.ReplaceAll(s, "\r\n", `\/n`)
	// Теперь заменим оставшиеся одиночные "\n":
	return strings.ReplaceAll(s, "\n", `\/n`)
}

func ReadLastNChars(path string, n int64) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("не удалось открыть файл: %w", err)
	}
	defer f.Close()

	// Узнаём размер файла
	info, err := f.Stat()
	if err != nil {
		return "", fmt.Errorf("не удалось получить инфо о файле: %w", err)
	}
	size := info.Size()

	// Если файл короче, чем n, позиционируемся в начало
	var offset int64
	if size > n {
		offset = size - n
	} else {
		offset = 0
	}

	// Переходим к offset
	_, err = f.Seek(offset, io.SeekStart)
	if err != nil {
		return "", fmt.Errorf("ошибка при seek: %w", err)
	}

	// Читаем оставшиеся данные
	buf, err := io.ReadAll(f)
	if err != nil {
		return "", fmt.Errorf("ошибка при чтении файла: %w", err)
	}

	return string(buf), nil
}

func GetLast100Msgs() (string, error) {
	chars, err := ReadLastNChars("/home/sinaibot/msgs", 200000)
	if err != nil {
		return "", err
	}
	fmt.Println(chars)
	lines := strings.Split(chars, "\n")

	return strings.Join(lines[len(lines)-100:], "\n"), nil
}

func revertarray(arr []string) []string {
	var newarr []string

	for i := len(arr) - 1; i >= 0; i-- {
		newarr = append(newarr, arr[i])
	}
	return newarr
}

var tokenRe = regexp.MustCompile(`\s*\S+`)

func accumulateAndEdit(c *tgClient, chatID int64, messageID int, username string, words <-chan string) {
	buffer := []string{}

	for raw := range words {
		fmt.Println(fmt.Sprintf("<%s>", raw))

		// raw — кусочек текста из стрима, например " Ka" или " kaz"
		tokens := tokenRe.FindAllString(raw, -1)
		for _, tok := range tokens {
			buffer = append(buffer, tok)
			if len(buffer)%2 == 0 {
				// токены уже содержат свои пробелы, склеиваем без доп. разделителя
				partial := strings.Join(buffer, "")
				_ = editMessage(c, chatID, messageID, fmt.Sprintf("%s, %s", username, partial))
			}
		}
	}

	// если нечётный остаток
	if len(buffer)%2 != 0 {
		full := strings.Join(buffer, "")
		_ = editMessage(c, chatID, messageID, fmt.Sprintf("%s, %s", username, full))
	}
}
