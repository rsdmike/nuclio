/*
Copyright 2017 The Nuclio Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package nucliozap

import (
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mgutz/ansi"
	"github.com/pavius/zap"
	"github.com/pavius/zap/zapcore"
)

type Level int8

const (
	DebugLevel  Level = Level(zapcore.DebugLevel)
	InfoLevel   Level = Level(zapcore.InfoLevel)
	WarnLevel   Level = Level(zapcore.WarnLevel)
	ErrorLevel  Level = Level(zapcore.ErrorLevel)
	DPanicLevel Level = Level(zapcore.DPanicLevel)
	PanicLevel  Level = Level(zapcore.PanicLevel)
	FatalLevel  Level = Level(zapcore.FatalLevel)
)

type writerWrapper struct {
	io.Writer
}

func (w writerWrapper) Sync() error {
	return nil
}

// concrete implementation of the nuclio logger interface, using zap
type NuclioZap struct {
	*zap.SugaredLogger
	atomicLevel       zap.AtomicLevel
	coloredLevelDebug string
	coloredLevelInfo  string
	coloredLevelWarn  string
	coloredLevelError string
	colorLoggerName   func(string) string
}

// create a configurable logger
func NewNuclioZap(name string,
	encoding string,
	sink io.Writer,
	errSink io.Writer,
	level Level) (*NuclioZap, error) {
	newNuclioZap := &NuclioZap{
		atomicLevel: zap.NewAtomicLevelAt(zapcore.Level(level)),
	}

	encoderConfig := newNuclioZap.getEncoderConfig(encoding)

	// create a sane configuration
	config := zap.Config{
		Level:              newNuclioZap.atomicLevel,
		Development:        true,
		Encoding:           encoding,
		EncoderConfig:      *encoderConfig,
		OutputWriters:      []zapcore.WriteSyncer{writerWrapper{sink}},
		ErrorOutputWriters: []zapcore.WriteSyncer{writerWrapper{errSink}},
		DisableStacktrace:  true,
	}

	newZapLogger, err := config.Build()
	if err != nil {
		return nil, err
	}

	newNuclioZap.SugaredLogger = newZapLogger.Sugar().Named(name)

	// initialize coloring by level
	newNuclioZap.initializeColors()

	return newNuclioZap, nil
}

// create a logger pre-configured for tests
func NewNuclioZapTest(name string) (*NuclioZap, error) {
	var loggerLevel Level

	if testing.Verbose() {
		loggerLevel = DebugLevel
	} else {
		loggerLevel = InfoLevel
	}

	return NewNuclioZapCmd(name, loggerLevel)
}

// create a logger pre-configured for commands
func NewNuclioZapCmd(name string, level Level) (*NuclioZap, error) {
	return NewNuclioZap(name, "console", os.Stdout, os.Stdout, level)
}

func GetLevelByName(levelName string) Level {
	switch levelName {
	case "info":
		return Level(zapcore.InfoLevel)
	case "warn":
		return Level(zapcore.WarnLevel)
	case "error":
		return Level(zapcore.ErrorLevel)
	case "dpanic":
		return Level(zapcore.DPanicLevel)
	case "panic":
		return Level(zapcore.PanicLevel)
	case "fatal":
		return Level(zapcore.FatalLevel)
	default:
		return Level(zapcore.DebugLevel)
	}
}

func (nz *NuclioZap) SetLevel(level Level) {
	nz.atomicLevel.SetLevel(zapcore.Level(level))
}

func (nz *NuclioZap) GetLevel() Level {
	return Level(nz.atomicLevel.Level())
}

func (nz *NuclioZap) Error(format interface{}, vars ...interface{}) {
	formatString, formatIsString := format.(string)
	if formatIsString {
		nz.SugaredLogger.Errorf(formatString, vars...)
	} else {
		nz.SugaredLogger.Error(format)
	}
}

func (nz *NuclioZap) ErrorWith(format interface{}, vars ...interface{}) {
	nz.SugaredLogger.Errorw(format.(string), vars...)
}

func (nz *NuclioZap) Warn(format interface{}, vars ...interface{}) {
	formatString, formatIsString := format.(string)
	if formatIsString {
		nz.SugaredLogger.Warnf(formatString, vars...)
	} else {
		nz.SugaredLogger.Warn(format)
	}
}

func (nz *NuclioZap) WarnWith(format interface{}, vars ...interface{}) {
	nz.SugaredLogger.Warnw(format.(string), vars...)
}

func (nz *NuclioZap) Info(format interface{}, vars ...interface{}) {
	formatString, formatIsString := format.(string)
	if formatIsString {
		nz.SugaredLogger.Infof(formatString, vars...)
	} else {
		nz.SugaredLogger.Info(format)
	}
}

func (nz *NuclioZap) InfoWith(format interface{}, vars ...interface{}) {
	nz.SugaredLogger.Infow(format.(string), vars...)
}

func (nz *NuclioZap) DebugWith(format interface{}, vars ...interface{}) {
	nz.SugaredLogger.Debugw(format.(string), vars...)
}

func (nz *NuclioZap) Debug(format interface{}, vars ...interface{}) {
	formatString, formatIsString := format.(string)
	if formatIsString {
		nz.SugaredLogger.Debugf(formatString, vars...)
	} else {
		nz.SugaredLogger.Debug(format)
	}
}

func (nz *NuclioZap) Flush() {
	nz.Sync()
}

func (nz *NuclioZap) GetChild(name string) interface{} {
	return &NuclioZap{SugaredLogger: nz.Named(name)}
}

func (nz *NuclioZap) encodeLoggerName(loggerName string, enc zapcore.PrimitiveArrayEncoder) {
	const maxLoggerNameLength = 25
	actualLoggerNameLength := len(loggerName)
	var encodedLoggerName string

	if actualLoggerNameLength >= maxLoggerNameLength {
		encodedLoggerName = loggerName[actualLoggerNameLength-maxLoggerNameLength:]

	} else {
		encodedLoggerName = strings.Repeat(" ", maxLoggerNameLength-actualLoggerNameLength) + loggerName
	}

	// just truncate
	enc.AppendString(nz.colorLoggerName(encodedLoggerName))
}

func (nz *NuclioZap) encodeStdoutLevel(level zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
	switch level {
	case zapcore.InfoLevel:
		enc.AppendString(nz.coloredLevelInfo)
		return
	case zapcore.WarnLevel:
		enc.AppendString(nz.coloredLevelWarn)
		return
	case zapcore.ErrorLevel:
		enc.AppendString(nz.coloredLevelError)
		return
	}

	enc.AppendString(nz.coloredLevelDebug)
}

func (nz *NuclioZap) encodeStdoutTime(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.Format("06.01.02 15:04:05.000"))
}

func (nz *NuclioZap) initializeColors() {
	nz.coloredLevelDebug = ansi.Color("(D)", "green")
	nz.coloredLevelInfo = ansi.Color("(I)", "blue")
	nz.coloredLevelWarn = ansi.Color("(W)", "yellow")
	nz.coloredLevelError = ansi.Color("(E)", "red")

	nz.colorLoggerName = ansi.ColorFunc("white")
}

func (nz *NuclioZap) getEncoderConfig(encoding string) *zapcore.EncoderConfig {
	if encoding == "console" {
		return &zapcore.EncoderConfig{
			TimeKey:          "time",
			LevelKey:         "level",
			NameKey:          "name",
			CallerKey:        "",
			MessageKey:       "message",
			StacktraceKey:    "stack",
			LineEnding:       zapcore.DefaultLineEnding,
			EncodeLevel:      nz.encodeStdoutLevel,
			EncodeTime:       nz.encodeStdoutTime,
			EncodeDuration:   zapcore.StringDurationEncoder,
			EncodeCaller:     func(zapcore.EntryCaller, zapcore.PrimitiveArrayEncoder) {},
			EncodeLoggerName: nz.encodeLoggerName,
		}
	} else {
		return &zapcore.EncoderConfig{
			TimeKey:          "time",
			LevelKey:         "level",
			NameKey:          "name",
			CallerKey:        "",
			MessageKey:       "message",
			StacktraceKey:    "stack",
			LineEnding:       ",",
			EncodeLevel:      zapcore.LowercaseLevelEncoder,
			EncodeTime:       zapcore.EpochMillisTimeEncoder,
			EncodeDuration:   zapcore.SecondsDurationEncoder,
			EncodeCaller:     func(zapcore.EntryCaller, zapcore.PrimitiveArrayEncoder) {},
			EncodeLoggerName: zapcore.FullLoggerNameEncoder,
		}
	}
}