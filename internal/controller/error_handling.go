/*
Copyright 2025.

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

package controller

import (
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
)

// ErrorType categorizes different types of errors for appropriate handling
type ErrorType int

const (
	// TransientError - temporary issues that benefit from controller-runtime's exponential backoff
	TransientError ErrorType = iota
	// ConfigurationError - user configuration issues that need longer delays
	ConfigurationError
	// DependencyError - missing dependencies that might resolve soon
	DependencyError
	// ValidationError - permanent validation failures that need longer delays
	ValidationError
)

// ErrorClassifier determines the appropriate handling strategy for different error types
type ErrorClassifier struct{}

// ClassifyError categorizes an error to determine the best requeue strategy
func (ec *ErrorClassifier) ClassifyError(err error) ErrorType {
	if err == nil {
		return TransientError // shouldn't happen, but safe default
	}

	errMsg := strings.ToLower(err.Error())

	// Transient errors - let controller-runtime handle with exponential backoff
	if errors.IsConflict(err) ||
		errors.IsServerTimeout(err) ||
		errors.IsTimeout(err) ||
		errors.IsTooManyRequests(err) ||
		errors.IsInternalError(err) ||
		strings.Contains(errMsg, "connection refused") ||
		strings.Contains(errMsg, "timeout") ||
		strings.Contains(errMsg, "network") {
		return TransientError
	}

	// Configuration errors - user input issues
	if errors.IsInvalid(err) ||
		strings.Contains(errMsg, "unsupported") ||
		strings.Contains(errMsg, "invalid configuration") ||
		strings.Contains(errMsg, "failed to marshal") ||
		strings.Contains(errMsg, "failed to generate config") {
		return ConfigurationError
	}

	// Dependency errors - missing resources that might be created soon
	if errors.IsNotFound(err) ||
		strings.Contains(errMsg, "not found") ||
		strings.Contains(errMsg, "does not support protocol") ||
		strings.Contains(errMsg, "no protocols defined") {
		return DependencyError
	}

	// Validation errors - schema/permission issues
	if errors.IsUnauthorized(err) ||
		errors.IsForbidden(err) ||
		strings.Contains(errMsg, "validation failed") ||
		strings.Contains(errMsg, "admission webhook") {
		return ValidationError
	}

	// Default to transient for unknown errors
	return TransientError
}

// GetRequeueStrategy returns the appropriate ctrl.Result based on error type
func (ec *ErrorClassifier) GetRequeueStrategy(err error, errorType ErrorType) (ctrl.Result, error) {
	switch errorType {
	case TransientError:
		// Let controller-runtime handle exponential backoff
		return ctrl.Result{}, err

	case ConfigurationError:
		// Configuration issues need longer delays - user needs to fix config
		return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil

	case DependencyError:
		// Dependencies might resolve soon - moderate delay
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil

	case ValidationError:
		// Validation issues usually need manual intervention - longer delay
		return ctrl.Result{RequeueAfter: 10 * time.Minute}, nil

	default:
		// Fallback to transient handling
		return ctrl.Result{}, err
	}
}

// HandleReconcileError provides intelligent error handling for reconcile operations
func (ec *ErrorClassifier) HandleReconcileError(err error) (ctrl.Result, error) {
	if err == nil {
		return ctrl.Result{}, nil
	}

	errorType := ec.ClassifyError(err)
	return ec.GetRequeueStrategy(err, errorType)
}

// IsRetriableError checks if an error should trigger a retry
func (ec *ErrorClassifier) IsRetriableError(err error) bool {
	if err == nil {
		return false
	}

	// Don't retry certain permanent errors
	errMsg := strings.ToLower(err.Error())
	permanentErrors := []string{
		"unsupported gateway provider",
		"invalid domain format",
		"missing required field",
	}

	for _, permanent := range permanentErrors {
		if strings.Contains(errMsg, permanent) {
			return false
		}
	}

	return true
}

// NewErrorClassifier creates a new error classifier
func NewErrorClassifier() *ErrorClassifier {
	return &ErrorClassifier{}
}
