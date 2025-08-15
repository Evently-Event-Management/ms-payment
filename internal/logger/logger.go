package logger

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/fatih/color"
)

type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
	FATAL
)

type LogEntry struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Category  string `json:"category"`
	Message   string `json:"message"`
	File      string `json:"file,omitempty"`
	Line      int    `json:"line,omitempty"`
}

type Logger struct {
	fileLogger   *log.Logger
	logFile      *os.File
	colorEnabled bool
}

func NewLogger() *Logger {
	// Create logs directory if it doesn't exist
	if err := os.MkdirAll("logs", 0755); err != nil {
		log.Fatal("Failed to create logs directory:", err)
	}

	// Create log file with timestamp
	timestamp := time.Now().Format("2006-01-02")
	logFileName := fmt.Sprintf("logs/payment-gateway-%s.log", timestamp)
	
	logFile, err := os.OpenFile(logFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatal("Failed to create log file:", err)
	}

	// Create multi-writer for both file and stdout
	multiWriter := io.MultiWriter(logFile, os.Stdout)
	fileLogger := log.New(multiWriter, "", 0)

	logger := &Logger{
		fileLogger:   fileLogger,
		logFile:      logFile,
		colorEnabled: true,
	}

	// Log startup message
	logger.Info("LOGGER", "Enhanced logging system initialized")
	logger.Info("LOGGER", fmt.Sprintf("Log file: %s", logFileName))

	return logger
}

func (l *Logger) log(level LogLevel, category, message string) {
	// Get caller information
	_, file, line, ok := runtime.Caller(2)
	if ok {
		file = filepath.Base(file)
	}

	entry := LogEntry{
		Timestamp: time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
		Level:     l.levelToString(level),
		Category:  strings.ToUpper(category),
		Message:   message,
		File:      file,
		Line:      line,
	}

	// Format for terminal output with colors
	terminalOutput := l.formatTerminalOutput(entry)
	
	// Format for file output (JSON)
	jsonOutput := l.formatJSONOutput(entry)

	// Write to terminal (colored)
	fmt.Print(terminalOutput)
	
	// Write to file (JSON format)
	if l.logFile != nil {
		l.logFile.WriteString(jsonOutput + "\n")
	}
}

func (l *Logger) formatTerminalOutput(entry LogEntry) string {
	timestamp := entry.Timestamp[11:19] // Extract time part
	
	var levelColor, categoryColor *color.Color
	
	switch entry.Level {
	case "DEBUG":
		levelColor = color.New(color.FgCyan)
		categoryColor = color.New(color.FgCyan, color.Bold)
	case "INFO":
		levelColor = color.New(color.FgGreen)
		categoryColor = color.New(color.FgGreen, color.Bold)
	case "WARN":
		levelColor = color.New(color.FgYellow)
		categoryColor = color.New(color.FgYellow, color.Bold)
	case "ERROR":
		levelColor = color.New(color.FgRed)
		categoryColor = color.New(color.FgRed, color.Bold)
	case "FATAL":
		levelColor = color.New(color.FgRed, color.Bold)
		categoryColor = color.New(color.FgRed, color.Bold)
	default:
		levelColor = color.New(color.FgWhite)
		categoryColor = color.New(color.FgWhite, color.Bold)
	}

	// Create formatted output
	timeStr := color.New(color.FgBlue).Sprintf("%s", timestamp)
	levelStr := levelColor.Sprintf("%-5s", entry.Level)
	categoryStr := categoryColor.Sprintf("[%-10s]", entry.Category)
	messageStr := entry.Message
	
	if entry.File != "" && entry.Line > 0 {
		fileInfo := color.New(color.FgMagenta).Sprintf(" (%s:%d)", entry.File, entry.Line)
		return fmt.Sprintf("%s %s %s %s%s\n", timeStr, levelStr, categoryStr, messageStr, fileInfo)
	}
	
	return fmt.Sprintf("%s %s %s %s\n", timeStr, levelStr, categoryStr, messageStr)
}

func (l *Logger) formatJSONOutput(entry LogEntry) string {
	jsonBytes, _ := json.Marshal(entry)
	return string(jsonBytes)
}

func (l *Logger) levelToString(level LogLevel) string {
	switch level {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	case FATAL:
		return "FATAL"
	default:
		return "INFO"
	}
}

// Public logging methods
func (l *Logger) Debug(category, message string) {
	l.log(DEBUG, category, message)
}

func (l *Logger) Info(category, message string) {
	l.log(INFO, category, message)
}

func (l *Logger) Warn(category, message string) {
	l.log(WARN, category, message)
}

func (l *Logger) Error(category, message string) {
	l.log(ERROR, category, message)
}

func (l *Logger) Fatal(category, message string) {
	l.log(FATAL, category, message)
	os.Exit(1)
}

// Specialized logging methods for different components
func (l *Logger) LogPayment(action, paymentID, message string) {
	l.Info("PAYMENT", fmt.Sprintf("[%s] %s - %s", action, paymentID, message))
}

func (l *Logger) LogAPI(method, path, status, duration string) {
	l.Info("API", fmt.Sprintf("%s %s - %s (%s)", method, path, status, duration))
}

func (l *Logger) LogKafka(action, topic, message string) {
	l.Info("KAFKA", fmt.Sprintf("[%s] %s - %s", action, topic, message))
}

func (l *Logger) LogProcess(processName, message string) {
	l.Info("PROCESS", fmt.Sprintf("[%s] %s", processName, message))
}

func (l *Logger) LogDatabase(operation, table, message string) {
	l.Info("DATABASE", fmt.Sprintf("[%s] %s - %s", operation, table, message))
}

func (l *Logger) LogSecurity(event, message string) {
	l.Warn("SECURITY", fmt.Sprintf("[%s] %s", event, message))
}

func (l *Logger) Close() {
	if l.logFile != nil {
		l.Info("LOGGER", "Closing log file")
		l.logFile.Close()
	}
}
