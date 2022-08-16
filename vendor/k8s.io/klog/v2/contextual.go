/*
Copyright 2021 The Kubernetes Authors.

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

package klog

import (
	"context"

	"github.com/go-logr/logr"
)

// This file provides the implementation of
// https://github.com/kubernetes/enhancements/tree/master/keps/sig-instrumentation/1602-structured-logging
//
// SetLogger and ClearLogger were originally added to klog.go and got moved
// here. Contextual logging adds a way to retrieve a Logger for direct logging
// without the logging calls in klog.go.
//
// The global variables are expected to be modified only during sequential
// parts of a program (init, serial tests) and therefore are not protected by
// mutex locking.

var (
<<<<<<< HEAD
=======
	// contextualLoggingEnabled controls whether contextual logging is
	// active. Disabling it may have some small performance benefit.
	contextualLoggingEnabled = true

	// globalLogger is the global Logger chosen by users of klog, nil if
	// none is available.
	globalLogger *Logger

	// globalLoggerOptions contains the options that were supplied for
	// globalLogger.
	globalLoggerOptions loggerOptions

>>>>>>> 268252f ( [WIP] Add support ImageDigest,TagMirrorSet CRDs)
	// klogLogger is used as fallback for logging through the normal klog code
	// when no Logger is set.
	klogLogger logr.Logger = logr.New(&klogger{})
)

// SetLogger sets a Logger implementation that will be used as backing
// implementation of the traditional klog log calls. klog will do its own
// verbosity checks before calling logger.V().Info. logger.Error is always
// called, regardless of the klog verbosity settings.
//
// If set, all log lines will be suppressed from the regular output, and
// redirected to the logr implementation.
// Use as:
//   ...
//   klog.SetLogger(zapr.NewLogger(zapLog))
//
// To remove a backing logr implemention, use ClearLogger. Setting an
// empty logger with SetLogger(logr.Logger{}) does not work.
//
// Modifying the logger is not thread-safe and should be done while no other
// goroutines invoke log calls, usually during program initialization.
func SetLogger(logger logr.Logger) {
	SetLoggerWithOptions(logger)
}

// SetLoggerWithOptions is a more flexible version of SetLogger. Without
// additional options, it behaves exactly like SetLogger. By passing
// ContextualLogger(true) as option, it can be used to set a logger that then
// will also get called directly by applications which retrieve it via
// FromContext, Background, or TODO.
//
// Supporting direct calls is recommended because it avoids the overhead of
// routing log entries through klogr into klog and then into the actual Logger
// backend.
func SetLoggerWithOptions(logger logr.Logger, opts ...LoggerOption) {
<<<<<<< HEAD
	logging.logger = &logger
	logging.loggerOptions = loggerOptions{}
	for _, opt := range opts {
		opt(&logging.loggerOptions)
=======
	globalLogger = &logger
	globalLoggerOptions = loggerOptions{}
	for _, opt := range opts {
		opt(&globalLoggerOptions)
>>>>>>> 268252f ( [WIP] Add support ImageDigest,TagMirrorSet CRDs)
	}
}

// ContextualLogger determines whether the logger passed to
// SetLoggerWithOptions may also get called directly. Such a logger cannot rely
// on verbosity checking in klog.
func ContextualLogger(enabled bool) LoggerOption {
	return func(o *loggerOptions) {
		o.contextualLogger = enabled
	}
}

// FlushLogger provides a callback for flushing data buffered by the logger.
func FlushLogger(flush func()) LoggerOption {
	return func(o *loggerOptions) {
		o.flush = flush
	}
}

// LoggerOption implements the functional parameter paradigm for
// SetLoggerWithOptions.
type LoggerOption func(o *loggerOptions)

type loggerOptions struct {
	contextualLogger bool
	flush            func()
}

// ClearLogger removes a backing Logger implementation if one was set earlier
// with SetLogger.
//
// Modifying the logger is not thread-safe and should be done while no other
// goroutines invoke log calls, usually during program initialization.
func ClearLogger() {
<<<<<<< HEAD
	logging.logger = nil
	logging.loggerOptions = loggerOptions{}
=======
	globalLogger = nil
	globalLoggerOptions = loggerOptions{}
>>>>>>> 268252f ( [WIP] Add support ImageDigest,TagMirrorSet CRDs)
}

// EnableContextualLogging controls whether contextual logging is enabled.
// By default it is enabled. When disabled, FromContext avoids looking up
// the logger in the context and always returns the global logger.
// LoggerWithValues, LoggerWithName, and NewContext become no-ops
// and return their input logger respectively context. This may be useful
// to avoid the additional overhead for contextual logging.
//
// This must be called during initialization before goroutines are started.
func EnableContextualLogging(enabled bool) {
<<<<<<< HEAD
	logging.contextualLoggingEnabled = enabled
=======
	contextualLoggingEnabled = enabled
>>>>>>> 268252f ( [WIP] Add support ImageDigest,TagMirrorSet CRDs)
}

// FromContext retrieves a logger set by the caller or, if not set,
// falls back to the program's global logger (a Logger instance or klog
// itself).
func FromContext(ctx context.Context) Logger {
<<<<<<< HEAD
	if logging.contextualLoggingEnabled {
=======
	if contextualLoggingEnabled {
>>>>>>> 268252f ( [WIP] Add support ImageDigest,TagMirrorSet CRDs)
		if logger, err := logr.FromContext(ctx); err == nil {
			return logger
		}
	}

	return Background()
}

// TODO can be used as a last resort by code that has no means of
// receiving a logger from its caller. FromContext or an explicit logger
// parameter should be used instead.
func TODO() Logger {
	return Background()
}

// Background retrieves the fallback logger. It should not be called before
// that logger was initialized by the program and not by code that should
// better receive a logger via its parameters. TODO can be used as a temporary
// solution for such code.
func Background() Logger {
<<<<<<< HEAD
	if logging.loggerOptions.contextualLogger {
		// Is non-nil because logging.loggerOptions.contextualLogger is
		// only true if a logger was set.
		return *logging.logger
=======
	if globalLoggerOptions.contextualLogger {
		// Is non-nil because globalLoggerOptions.contextualLogger is
		// only true if a logger was set.
		return *globalLogger
>>>>>>> 268252f ( [WIP] Add support ImageDigest,TagMirrorSet CRDs)
	}

	return klogLogger
}

// LoggerWithValues returns logger.WithValues(...kv) when
// contextual logging is enabled, otherwise the logger.
func LoggerWithValues(logger Logger, kv ...interface{}) Logger {
<<<<<<< HEAD
	if logging.contextualLoggingEnabled {
=======
	if contextualLoggingEnabled {
>>>>>>> 268252f ( [WIP] Add support ImageDigest,TagMirrorSet CRDs)
		return logger.WithValues(kv...)
	}
	return logger
}

// LoggerWithName returns logger.WithName(name) when contextual logging is
// enabled, otherwise the logger.
func LoggerWithName(logger Logger, name string) Logger {
<<<<<<< HEAD
	if logging.contextualLoggingEnabled {
=======
	if contextualLoggingEnabled {
>>>>>>> 268252f ( [WIP] Add support ImageDigest,TagMirrorSet CRDs)
		return logger.WithName(name)
	}
	return logger
}

// NewContext returns logr.NewContext(ctx, logger) when
// contextual logging is enabled, otherwise ctx.
func NewContext(ctx context.Context, logger Logger) context.Context {
<<<<<<< HEAD
	if logging.contextualLoggingEnabled {
=======
	if contextualLoggingEnabled {
>>>>>>> 268252f ( [WIP] Add support ImageDigest,TagMirrorSet CRDs)
		return logr.NewContext(ctx, logger)
	}
	return ctx
}
