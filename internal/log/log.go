// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package log configures the logger.
package log

import (
	"fmt"
	"log"
	"strings"

	"github.com/siderolabs/go-kmsg"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// InitLogger initializes the logger with the given options.
func InitLogger(debug, logToKmsg bool) (*zap.Logger, error) {
	var (
		encoder zapcore.Encoder
		level   zapcore.Level
	)

	if debug {
		encoderConfig := zap.NewDevelopmentEncoderConfig()
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder

		encoder = zapcore.NewConsoleEncoder(encoderConfig)
		level = zap.DebugLevel
	} else {
		encoderConfig := zap.NewProductionEncoderConfig()

		encoder = zapcore.NewJSONEncoder(encoderConfig)
		level = zap.InfoLevel
	}

	stdErrWriter, closeStdErrWriter, err := zap.Open("stderr")
	if err != nil {
		closeStdErrWriter()

		return nil, err
	}

	writer := stdErrWriter

	if logToKmsg {
		stdLogger := log.New(nil, "", 0)

		if err = kmsg.SetupLogger(stdLogger, "[ext-talos-metal-agent]", nil); err != nil {
			return nil, fmt.Errorf("failed to set kmsg debug logger: %w", err)
		}

		kmsgWriter := loggerWriter{
			logger: stdLogger,
		}

		kmsgWriteSyncer := zapcore.AddSync(&kmsgWriter)

		writer = zap.CombineWriteSyncers(stdErrWriter, kmsgWriteSyncer)
	}

	core := zapcore.NewCore(encoder, writer, level)
	logger := zap.New(core, zap.AddStacktrace(zapcore.ErrorLevel))

	return logger, nil
}

// loggerWriter wraps a log.Logger to implement the io.Writer interface.
//
// By doing this instead of using log.Logger's internal writer, we benefit from the log.Logger's prefixing and other formatting options.
type loggerWriter struct {
	logger *log.Logger
	buf    strings.Builder
}

func (l *loggerWriter) Write(p []byte) (n int, err error) {
	const maxLineLength = 4096

	for _, b := range p {
		l.buf.WriteByte(b)

		if b == '\n' || l.buf.Len() > maxLineLength {
			l.logger.Print(l.buf.String())
			l.buf.Reset()
		}
	}

	return len(p), nil
}
