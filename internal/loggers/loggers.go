package loggers

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/gookit/slog"
	"github.com/gookit/slog/handler"
)

// Глобальные переменные с начальными значениями.
var (
	logConsole  = true
	IsDebugMode = true
	IsInfoMode  = true
	IsWarnMode  = true
)

// SetLogConsole устанавливает значение для logConsole.
func SetLogConsole(value bool) {
	logConsole = value
}

// SetIsDebugMode устанавливает значение для IsDebugMode.
func SetIsDebugMode(value bool) {
	IsDebugMode = value
}

// SetIsInfoMode устанавливает значение для IsInfoMode.
func SetIsInfoMode(value bool) {
	IsInfoMode = value
}

// SetIsWarnMode устанавливает значение для IsWarnMode.
func SetIsWarnMode(value bool) {
	IsWarnMode = value
}

// CustomFormatter - кастомный формат для изменения формата даты.
type CustomFormatter struct{}

// Format реализует метод интерфейса slog.Formatter.
func (f *CustomFormatter) Format(record *slog.Record) ([]byte, error) {
	//formattedTime := record.Time.Format("15:04:05 02.01.2006")
	caller := record.Caller
	fileName := filepath.Base(caller.File) // Получаем только имя файла
	funcName := getFunctionName(caller.PC) // Получаем имя функции
	logMessage := fmt.Sprintf("[%s] [%s] [%s:%d,%s] %s\n", record.Level.String(), record.Time.Format("2006-01-02 15:04:05"), fileName, caller.Line, funcName, record.Message)
	return []byte(logMessage), nil
}

// getFunctionName возвращает короткое имя функции по Program Counter (PC).
func getFunctionName(pc uintptr) string {
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return "unknown"
	}

	// Извлекаем только короткое имя функции
	return filepath.Base(fn.Name())
}

// SetupLogger настраивает и возвращает логгер.
func SetupLogger() *slog.Logger {
	// Создаем логгер
	logger := slog.New()

	// Устанавливаем форматтер
	formatter := &CustomFormatter{}

	if logConsole {
		// Создаем хэндлер для вывода в консоль с кастомным форматтером
		consoleHandler := handler.NewConsoleHandler(getLogLevels())
		consoleHandler.SetFormatter(formatter)
		logger.AddHandler(consoleHandler)
	} else {
		// Получаем текущее время и дату
		now := time.Now()

		// Определим путь к файлу лога с учетом текущей даты в формате DD-MM-YYYY
		logFilePath := fmt.Sprintf("log/error-%s.log", now.Format("02-01-2006"))

		// Создаем директорию для логов, если её нет
		err := os.MkdirAll(filepath.Dir(logFilePath), 0755)
		if err != nil {
			panic(fmt.Sprintf("Error creating log directory: %v", err))
		}

		// Открываем файл для записи логов
		logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			panic(fmt.Sprintf("Error opening log file: %v", err))
		}

		// Создаем хэндлер для записи в файл с кастомным форматтером
		fileHandler := handler.NewIOWriterHandler(logFile, getLogLevels())
		fileHandler.SetFormatter(formatter)
		logger.AddHandler(fileHandler)
	}

	return logger
}

func getLogLevels() []slog.Level {
	levels := []slog.Level{slog.ErrorLevel, slog.FatalLevel}

	if IsWarnMode {
		levels = append(levels, slog.WarnLevel)
	}
	if IsInfoMode {
		levels = append(levels, slog.InfoLevel)
	}
	if IsDebugMode {
		levels = append(levels, slog.DebugLevel)
	}

	return levels
}
