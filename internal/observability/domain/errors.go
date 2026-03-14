package domain

import "fmt"

// ErrAlertRuleNotFound is returned when an alert rule cannot be found.
var ErrAlertRuleNotFound = fmt.Errorf("alert rule not found")

// ErrAlertNotFound is returned when an alert cannot be found.
var ErrAlertNotFound = fmt.Errorf("alert not found")
