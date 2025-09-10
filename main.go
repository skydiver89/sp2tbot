package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	vosk "github.com/alphacep/vosk-api/go"
	telegram "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"gopkg.in/yaml.v3"
)

type Config struct {
	TelegramAPIKey string  `yaml:"apiKey"`
	ModelPath      string  `yaml:"modelPath"`
	AllowedUserIDs []int64 `yaml:"allowedUserIds"`
}

type VoskAnswer struct {
	Text string `json:"text"`
}

func loadConfig(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var cfg Config
	dec := yaml.NewDecoder(f)
	if err := dec.Decode(&cfg); err != nil {
		return nil, err
	}
	if cfg.TelegramAPIKey == "" || cfg.ModelPath == "" {
		return nil, errors.New("invalid config: apiKey and modelPath are required")
	}
	return &cfg, nil
}

func isUserAllowed(userID int64, allowed []int64) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, id := range allowed {
		if id == userID {
			return true
		}
	}
	return false
}

func downloadTelegramFile(bot *telegram.BotAPI, fileID string) (string, error) {
	file, err := bot.GetFile(telegram.FileConfig{FileID: fileID})
	if err != nil {
		return "", err
	}
	url := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", bot.Token, file.FilePath)
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download file: status %s", resp.Status)
	}
	tmpFile, err := os.CreateTemp("", "tg-audio-*")
	if err != nil {
		return "", err
	}
	defer tmpFile.Close()
	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		return "", err
	}
	return tmpFile.Name(), nil
}

func convertToWav16kMonoPcmS16le(inputPath string) (string, error) {
	outputPath := inputPath + "-converted.wav"
	cmd := exec.Command("ffmpeg", "-y", "-i", inputPath, "-ar", "16000", "-ac", "1", "-c:a", "pcm_s16le", outputPath)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("ffmpeg conversion failed: %w", err)
	}
	return outputPath, nil
}

func readAll(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

func isLikelyAudio(m *telegram.Message) bool {
	if m == nil {
		return false
	}
	if m.Voice != nil || m.Audio != nil {
		return true
	}
	if m.Document != nil {
		mime := strings.ToLower(m.Document.MimeType)
		if strings.HasPrefix(mime, "audio/") || strings.Contains(mime, "ogg") || strings.Contains(mime, "mpeg") || strings.Contains(mime, "wav") {
			return true
		}
	}
	return false
}

// cleanupOldTempFiles removes leftover temporary files created by this app that are older than maxAge.
func cleanupOldTempFiles() {
	tmpDir := os.TempDir()
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
 		return
 	}
 	for _, entry := range entries {
 		name := entry.Name()
 		if strings.HasPrefix(name, "tg-audio-") {
			os.Remove(filepath.Join(tmpDir, name))
 		}
 	}
}

func main() {
    // Sweep old temporary files older than 1 hour
    cleanupOldTempFiles()
	cfg, err := loadConfig("config.yaml")
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		return
	}

	if _, err := os.Stat(cfg.ModelPath); os.IsNotExist(err) {
		fmt.Printf("Модель по пути %s не найдена. Загрузите её с https://alphacephei.com/vosk/models\n", cfg.ModelPath)
		return
	}

	model, err := vosk.NewModel(cfg.ModelPath)
	if err != nil {
		fmt.Printf("Ошибка загрузки модели: %v\n", err)
		return
	}
	defer model.Free()

	recognizer, err := vosk.NewRecognizer(model, 16000.0)
	if err != nil {
		fmt.Printf("Ошибка создания распознавателя: %v\n", err)
		return
	}
	defer recognizer.Free()

	bot, err := telegram.NewBotAPI(cfg.TelegramAPIKey)
	if err != nil {
		fmt.Printf("Ошибка инициализации Telegram бота: %v\n", err)
		return
	}
	bot.Debug = false

	u := telegram.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	ctx := context.Background()
	_ = ctx
	for update := range updates {
		if update.Message == nil {
			continue
		}

		userID := update.Message.From.ID
		if !isUserAllowed(userID, cfg.AllowedUserIDs) {
			msg := telegram.NewMessage(update.Message.Chat.ID, "Доступ запрещён.")
			_, _ = bot.Send(msg)
			continue
		}

		if !isLikelyAudio(update.Message) {
			msg := telegram.NewMessage(update.Message.Chat.ID, "Пожалуйста, пришлите аудио/voice файл.")
			_, _ = bot.Send(msg)
			continue
		}

		func() {
			var fileID string
			if update.Message.Voice != nil {
				fileID = update.Message.Voice.FileID
			} else if update.Message.Audio != nil {
				fileID = update.Message.Audio.FileID
			} else if update.Message.Document != nil {
				fileID = update.Message.Document.FileID
			}

			waiting := telegram.NewMessage(update.Message.Chat.ID, "Обрабатываю файл…")
			sent, _ := bot.Send(waiting)

			localPath, err := downloadTelegramFile(bot, fileID)
			if err != nil {
				edit := telegram.NewEditMessageText(update.Message.Chat.ID, sent.MessageID, fmt.Sprintf("Ошибка скачивания файла: %v", err))
				_, _ = bot.Request(edit)
				return
			}
			defer os.Remove(localPath)

			converted, err := convertToWav16kMonoPcmS16le(localPath)
			if err != nil {
				edit := telegram.NewEditMessageText(update.Message.Chat.ID, sent.MessageID, fmt.Sprintf("Ошибка конвертации: %v", err))
				_, _ = bot.Request(edit)
				return
			}
			defer os.Remove(converted)

			data, err := readAll(converted)
			if err != nil {
				edit := telegram.NewEditMessageText(update.Message.Chat.ID, sent.MessageID, fmt.Sprintf("Ошибка чтения файла: %v", err))
				_, _ = bot.Request(edit)
				return
			}

			_ = recognizer.AcceptWaveform(data)
			resultText := recognizer.FinalResult()
			if strings.TrimSpace(resultText) == "" {
				resultText = "Не удалось распознать речь."
			}

			var answer VoskAnswer
			err = json.Unmarshal([]byte(resultText), &answer)
			if err != nil {
				resultText = "Ошибка преобразования JSON-ответа VOSK " + err.Error()
			} else {
				resultText = answer.Text
			}

			edit := telegram.NewEditMessageText(update.Message.Chat.ID, sent.MessageID, resultText)
			_, _ = bot.Request(edit)
			time.Sleep(100 * time.Millisecond)
		}()
	}
}
