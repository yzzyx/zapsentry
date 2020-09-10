// Copyright (c) 2019 Elias Norberg
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
package zapsentry

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
	"go.uber.org/zap/zapcore"
)

// DefaultTagPrefix defines the default prefix that is used to mark a zap field as a sentry tag
// All fields that do not have this tag will be added as 'extra'
// Setting a prefix in the config variable when creating a new core replaces this value in that specific
// instance of the core
const DefaultTagPrefix = "#"

// Errors that might occur when logging to sentry
var (
	ErrFlushTimeout             = errors.New("sentry Flush() timeout occurred")
	ErrClientOrScopeUnavailable = errors.New("sentry client or scope is unavailable")
)

// Translation table from zap levels to sentry levels.
// Note that Panic and Fatal-levels are logged as errors - we don't want the
// Sentry core to panic or quit on us, and will instead let zap handle that.
var zapToSentryLevels = map[zapcore.Level]sentry.Level{
	zapcore.DebugLevel:  sentry.LevelDebug,
	zapcore.InfoLevel:   sentry.LevelInfo,
	zapcore.WarnLevel:   sentry.LevelWarning,
	zapcore.ErrorLevel:  sentry.LevelError,
	zapcore.DPanicLevel: sentry.LevelError,
	zapcore.PanicLevel:  sentry.LevelError,
	zapcore.FatalLevel:  sentry.LevelError,
}

// SentryCore defines a zapcore.Core that logs information to Sentry
type SentryCore struct {
	zapcore.LevelEnabler
	level  zapcore.Level
	hub    *sentry.Hub
	fields []zapcore.Field

	// Prefix used in field names to denote that they should be marked as a sentry tag.
	// If not specified, DefaultTagPrefix will be used
	TagPrefix string
}

// NewSentryCore creates a new zapcore.Core  that logs information to Sentry
func NewCore(hub *sentry.Hub, enab zapcore.LevelEnabler, fields ...zapcore.Field) (zapcore.Core, error) {
	core := &SentryCore{}
	core.hub = hub
	core.LevelEnabler = enab
	core.fields = fields
	return core, nil
}

// With adds structured context to the Core
func (core *SentryCore) With(fields []zapcore.Field) zapcore.Core {
	// Clone core.
	clone := *core

	// Clone and append fields.
	clone.fields = make([]zapcore.Field, len(core.fields)+len(fields))
	copy(clone.fields, core.fields)
	copy(clone.fields[len(core.fields):], fields)

	// Done.
	return &clone
}

// Check determines whether the supplied Entry should be logged
func (core *SentryCore) Check(entry zapcore.Entry, checked *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if core.Enabled(entry.Level) {
		return checked.AddCore(entry, core)
	}
	return checked
}

// Write serializes the Entry and any Fields supplied at the log site and writes them to the sentry
// client used when creating the SentryCore instance
func (core *SentryCore) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	event := sentry.NewEvent()
	event.Message = entry.Message
	event.Timestamp = entry.Time
	event.Level = zapToSentryLevels[entry.Level]
	event.Logger = entry.LoggerName

	enc := zapcore.NewMapObjectEncoder()

	for k := range core.fields {
		core.fields[k].AddTo(enc)
	}

	for k := range fields {
		fields[k].AddTo(enc)
	}

	tags := make(map[string]string)
	extra := make(map[string]interface{})

	tagPrefix := core.TagPrefix
	if tagPrefix == "" {
		tagPrefix = DefaultTagPrefix
	}

	for key, value := range enc.Fields {
		if strings.HasPrefix(key, tagPrefix) {
			if v, ok := value.(string); ok {
				tags[key[1:]] = v
			} else {
				tags[key[1:]] = fmt.Sprintf("%v", value)
			}
			continue
		}
		extra[key] = value
	}

	if len(tags) > 0 {
		event.Tags = tags
	}

	if len(extra) > 0 {
		event.Extra = extra
	}

	eventID := core.hub.CaptureEvent(event)
	if eventID == nil {
		return ErrClientOrScopeUnavailable
	}

	return nil
}

// Sync flushes buffered logs (if any)
func (core *SentryCore) Sync() error {
	flushed := core.hub.Flush(time.Second * 5)
	if !flushed {
		return ErrFlushTimeout
	}
	return nil
}
