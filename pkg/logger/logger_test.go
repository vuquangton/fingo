package logger_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"fingo/pkg/logger"
)

func TestLogger_StructuredJSONOutput(t *testing.T) {
	// Arrange
	var buf bytes.Buffer
	log := logger.New(logger.Config{
		Level:  slog.LevelInfo,
		Writer: &buf,
		Format: logger.FormatJSON,
	})

	ctx := context.Background()

	// Act
	log.Info(ctx, "voucher created",
		slog.String("voucher_no", "PK0001"),
		slog.Float64("amount", 1500000),
	)

	// Assert
	raw := strings.TrimSpace(buf.String())
	if raw == "" {
		t.Fatalf("expected non-empty log output")
	}

	var entry map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &entry); err != nil {
		t.Fatalf("invalid json log output: %v. Raw: %s", err, raw)
	}

	if entry["msg"] != "voucher created" {
		t.Errorf("expected msg 'voucher created', got %v", entry["msg"])
	}
	if entry["level"] != "INFO" {
		t.Errorf("expected level 'INFO', got %v", entry["level"])
	}
	if entry["voucher_no"] != "PK0001" {
		t.Errorf("expected voucher_no 'PK0001', got %v", entry["voucher_no"])
	}
}

func TestLogger_RespectsLogLevel(t *testing.T) {
	// Arrange: Log level is WARN, DEBUG should not be emitted
	var buf bytes.Buffer
	log := logger.New(logger.Config{
		Level:  slog.LevelWarn,
		Writer: &buf,
		Format: logger.FormatJSON,
	})

	ctx := context.Background()

	// Act
	log.Debug(ctx, "trace diagnostic message")

	// Assert
	if buf.Len() != 0 {
		t.Errorf("expected 0 bytes logged for debug message when level is WARN, got: %s", buf.String())
	}
}

func TestLogger_WithContextAttributes(t *testing.T) {
	// Arrange: Context with trace_id and user_id should be included automatically in logs
	var buf bytes.Buffer
	log := logger.New(logger.Config{
		Level:  slog.LevelInfo,
		Writer: &buf,
		Format: logger.FormatJSON,
	})

	ctx := logger.WithTraceID(context.Background(), "req-xyz-123")
	ctx = logger.WithUserID(ctx, "user-admin-01")

	// Act
	log.Info(ctx, "user logged in")

	// Assert
	var entry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed parsing log entry: %v", err)
	}

	if entry["trace_id"] != "req-xyz-123" {
		t.Errorf("expected trace_id 'req-xyz-123', got %v", entry["trace_id"])
	}
	if entry["user_id"] != "user-admin-01" {
		t.Errorf("expected user_id 'user-admin-01', got %v", entry["user_id"])
	}
}
